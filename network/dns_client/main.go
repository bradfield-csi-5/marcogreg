package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"
	"golang.org/x/sys/unix"
)

type Header struct {
	Id      uint16
	Flags   uint16
	QdCount uint16
	AnCount uint16
	NSCount uint16
	ARCount uint16
}

type Question struct {
	Qname  []byte
	Qtype  uint16
	Qclass uint16
}

type RR struct {
	Name     string
	Type     string
	Class    uint16
	Ttl      uint32
	RdLength uint16
	RData    string
}

var typeNames = map[uint16]string{
	1:  "A",
	2:  "NS",
	5:  "CNAME",
	6:  "SOA",
	15: "MX",
	16: "TXT",
}

func encodeHeader() ([]byte, error) {
	h := Header{Id: uint16(rand.Intn(math.MaxUint16)), Flags: 0x100, QdCount: 1, AnCount: 0, NSCount: 0, ARCount: 0}
	var buf bytes.Buffer

	err := binary.Write(&buf, binary.BigEndian, h)
	return buf.Bytes(), err
}

func encodeQname(d string) []byte {
	var buf bytes.Buffer
	dsplit := strings.Split(d, ".")
	for _, v := range dsplit {
		buf.WriteByte(byte(len(v)))
		buf.WriteString(v)
	}
	buf.WriteByte(0)
	return buf.Bytes()
}

func encodeQuestion(d string) ([]byte, error) {
	qname := encodeQname(d)

	q := Question{Qname: qname, Qtype: 1, Qclass: 1}

	var buf bytes.Buffer
	buf.Write(qname)
	err := binary.Write(&buf, binary.BigEndian, q.Qtype)
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.BigEndian, q.Qclass)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), err
}

func encodeQuery(domain string) ([]byte, error) {
	headers, err := encodeHeader()
	if err != nil {
		return nil, err
	}

	question, err := encodeQuestion(domain)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(headers)
	buf.Write(question)
	return buf.Bytes(), err
}

func decodeHeader(resp []byte, offset int) (Header, int) {
	offset += 12
	buf := bytes.NewReader(resp[:offset])
	var header Header
	binary.Read(buf, binary.BigEndian, &header)
	return header, offset
}

func parseName(b []byte, offset int) (string, int) {
	labels := []string{}
	for {
		cb := int(b[offset])
		if cb == 0 {
			break
		}

		if (cb & 0xc0) == 0xc0 {
			// follow pointer
			pointer := (cb & 0x3F << 8) | int(b[offset+1])
			result, _ := parseName(b, pointer)
			labels = append(labels, result)
			return strings.Join(labels, "."), offset + 2
		}

		label := string(b[offset+1 : offset+1+cb])
		labels = append(labels, label)
		offset += 1 + cb
	}
	return strings.Join(labels, "."), offset + 1
}

func parseRecordData(rdata uint32) string {
	parts := []string{}
	mask := uint32(0x000000ff)
	rshift := uint32(0)
	for i := 0; i < 4; i++ {
		parts = append(parts, strconv.Itoa(int((rdata&mask)>>rshift)))
		mask <<= uint(8)
		rshift += 8
	}
	slices.Reverse(parts)
	return strings.Join(parts, ".")
}

func decodeResponse(resp []byte) (RR, error) {
	offset := 0
	_, offset = decodeHeader(resp, offset)

	qName, offset := parseName(resp, offset)
	qtype := binary.BigEndian.Uint16(resp[offset:])
	offset += 2
	qclass := binary.BigEndian.Uint16(resp[offset:])
	offset += 2
	fmt.Println("Question: ", qName, qtype, qclass)

	answerName, offset := parseName(resp, offset)
	ansType := binary.BigEndian.Uint16(resp[offset:])
	offset += 2
	ansClass := binary.BigEndian.Uint16(resp[offset:])
	offset += 2
	ttl := binary.BigEndian.Uint32(resp[offset:])
	offset += 4
	rdlength := binary.BigEndian.Uint16(resp[offset:])
	offset += 2
	rdata := binary.BigEndian.Uint32(resp[offset:])
	offset += 4

	ip := parseRecordData(rdata)

	return RR{
		Name:     answerName,
		Type:     typeNames[ansType],
		Class:    ansClass,
		Ttl:      ttl,
		RdLength: rdlength,
		RData:    ip,
	}, nil
}

func main() {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		panic(err)
	}
	defer unix.Close(fd)

	query, err := encodeQuery(os.Args[1])
	if err != nil {
		fmt.Println("Error encoding query ", err)
		os.Exit(1)
	}

	sa := unix.SockaddrInet4{Port: 53, Addr: [4]byte{8, 8, 8, 8}}
	err = unix.Sendto(fd, query, 0, &sa)
	if err != nil {
		fmt.Println(err)
		return
	}

	respBuf := make([]byte, 128)
	_, _, err = unix.Recvfrom(fd, respBuf, 0)
	if err != nil {
		fmt.Println(err)
		return
	}

	response, err := decodeResponse(respBuf)
	fmt.Println("Answer: ", response.Name, response.Type, "IN", response.Ttl, response.RData)
}
