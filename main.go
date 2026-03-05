package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
)

func handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	parts := strings.Fields(requestLine)
	if len(parts) < 3 {
		return
	}

	path := parts[1]

	for {
		line, err := reader.ReadString('\n')
		if err != nil || line == "\r\n" {
			break
		}
	}

	body := "<html><body><h1>Hello! You requested: " + path + "</h1></body></html>"
	contentLength := len(body)

	response :=
		"HTTP/1.1 200 OK\r\n" +
			"Content-Type: text/html\r\n" +
			fmt.Sprintf("Content-Length: %d\r\n", contentLength) +
			"\r\n" +
			body

	conn.Write([]byte(response))
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Println("Listening on port 8080")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn)
	}
}
