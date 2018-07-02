package framework

import (
	"net"
	"fmt"
	"bufio"
)

type TcpServer struct {
}

func (this *TcpServer) Start(port int, handler func(net.Conn, chan string)) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		// handle error
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			// handle error
		}
		chann := make(chan string)
		for i := <-chann; ; {
			println(i)
		}
		go handler(conn, chann)
	}
}

func (thsi *TcpServer) Conn() {
	conn, err := net.Dial("tcp", "golang.org:80")
	if err != nil {
		// handle error
	}
	fmt.Fprint(conn, "GET / HTTP/1.0\r\n\r\n")
	status, err := bufio.NewReader(conn).ReadString('\n')
	// ...
	fmt.Println(status)
}
