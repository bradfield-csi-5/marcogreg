package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func handle(fd int) {
	buf := make([]byte, 4096)
	_, _, err := unix.Recvfrom(fd, buf, 0)
	check(err)

	resp := forward(buf)
	err = unix.Send(fd, resp, 0)
	check(err)
}

func forward(b []byte) []byte {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	defer unix.Close(fd)
	check(err)

	err = unix.Connect(fd, &unix.SockaddrInet4{Port: 8001, Addr: [4]byte{127, 0, 0, 1}})
	check(err)

	err = unix.Send(fd, b, 0)
	check(err)

	buf := make([]byte, 4096)
	_, _, err = unix.Recvfrom(fd, buf, 0)
	fmt.Println(string(buf))
	check(err)

	return buf
}

func main() {
	serverSocket, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	defer unix.Close(serverSocket)
	check(err)

	sa := unix.SockaddrInet4{Port: 8000, Addr: [4]byte{127, 0, 0, 1}}
	unix.SetsockoptInt(serverSocket, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
	err = unix.Bind(serverSocket, &sa)
	check(err)

	err = unix.Listen(serverSocket, 5)
	fmt.Println("Listening on ", sa.Addr, sa.Port)
	check(err)

	for {
		clientSocket, _, err := unix.Accept(serverSocket)
		if err != nil {
			fmt.Println("Error accepting new connection from queue ", err)
			continue
		}

		handle(clientSocket)

		unix.Close(clientSocket)
	}
}
