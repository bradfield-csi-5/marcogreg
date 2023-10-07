package main

import (
	"fmt"
	"strings"

	"golang.org/x/sys/unix"
)

var cache = make(map[string][]byte)

func main() {
	serverSocket, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	defer unix.Close(serverSocket)
	check(err)

	sa := unix.SockaddrInet4{Port: 8000, Addr: [4]byte{127, 0, 0, 1}}
	unix.SetsockoptInt(serverSocket, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
	err = unix.Bind(serverSocket, &sa)
	check(err)

	err = unix.Listen(serverSocket, unix.SOMAXCONN)
	fmt.Println("Listening on ", sa.Addr, sa.Port)
	check(err)

	for {
		clientSocket, _, err := unix.Accept(serverSocket)
		if err != nil {
			fmt.Println("Error accepting new connection from queue ", err)
			continue
		}

		handle(clientSocket)
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func handle(fd int) {
	defer unix.Close(fd)
	buf := make([]byte, 4096)
	_, _, err := unix.Recvfrom(fd, buf, 0)
	check(err)

	path := parsePathFromHttpReq(buf)
	cachedResp, cacheExist := getCache(path)

	if cacheExist {
		fmt.Println("Returned from cache ", path)
		unix.Send(fd, cachedResp, 0)
		return
	}

	resp := forward(buf)
	if err = unix.Send(fd, resp, 0); err != nil {
		fmt.Println("Error sending back to client ", err)
	}

	cache[path] = resp
}

func parsePathFromHttpReq(b []byte) string {
	decoded := string(b)
	return strings.Split(strings.Split(decoded, "\r\n")[0], " ")[1]
}

func getCache(path string) (resp []byte, found bool) {
	if record, found := cache[path]; found {
		return record, found
	}
	return nil, false
}

func forward(b []byte) []byte {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	defer unix.Close(fd)
	check(err)

	if err = unix.Connect(fd, &unix.SockaddrInet4{Port: 8001, Addr: [4]byte{127, 0, 0, 1}}); err != nil {
		fmt.Println("Error while connecting to server", err)
		return nil
	}

	err = unix.Send(fd, b, 0)
	check(err)

	buf := make([]byte, 512)
	resp := make([]byte, 0)
	total := 0
	for {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		check(err)

		if n == 0 {
			break
		}

		resp = append(resp, buf[:n]...)
		total += n
	}

	return resp
}
