package net

import (
		"net"
		"fmt"
		"time"
		"testing"
		"crypto/tls"
)

type TestServer struct {
	Startup 	<-chan net.Conn
	shutdown 	chan<- chan bool
	Recv 		<-chan []byte
}

type impTestServer struct {
	startup 	chan<- net.Conn
	shutdown 	<-chan chan bool
	recv 		chan<- []byte
}

func (t *TestServer) Close() {
	closer := make(chan bool)
	fmt.Println("sending closer")
	t.shutdown <- closer
	fmt.Println("waiting on close")
	<- closer
}

func TlsServer(port int, config *tls.Config, t *testing.T) TestServer {
	ln, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), config)
	if err != nil {
		t.Fatalf("error in listen %v", err)
	}
	return startAccept(ln, t)
}

func NetServer(port int, t *testing.T) TestServer {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatalf("error in listen %v", err)
	}
	return startAccept(ln, t)
}

func NonResponsiveServer(port int, t *testing.T) TestServer {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatalf("error in listen %v", err)
	}
	return stallAccept(ln, t)
}

func stallAccept(ln net.Listener, t *testing.T) TestServer {
	st := make(chan net.Conn)
	sh := make(chan chan bool)
	re := make(chan []byte)
	server := TestServer{st,sh,re}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			t.Fatalf("error in accept %v", err)
		}
		fmt.Println("accepted")
		closer := <- sh
		fmt.Println("closing staller")
		ln.Close()
		conn.Close()
		closer <- true
	}()
	return server
}

func startAccept(ln net.Listener, t *testing.T) TestServer {
	st := make(chan net.Conn)
	sh := make(chan chan bool)
	re := make(chan []byte)
	server := TestServer{st,sh,re}
	impServer := impTestServer{st,sh,re}
	go func() {
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			t.Fatalf("error in accept %v", err)
		}
		startServerConn(conn, impServer, t)
	}()
	return server
}

func startServerConn(conn net.Conn, server impTestServer, t *testing.T) {
	conn.SetReadDeadline(time.Now().Add(time.Second))
	server.startup <- conn
	fmt.Println("started")
	for {
		buffer := make([]byte, 1024)
		n, err := conn.Read(buffer)
		fmt.Println("read n bytes", n)
		if err != nil {
			fmt.Println("error in read", err)
			break
		} else {
			server.recv <- buffer[0:n]
		}
	}
	closer := <- server.shutdown
	conn.Close()
	closer <- true
}
