package resilient

import (
		"os"
		"fmt"
		"net"
		"testing"
		"crypto/tls"
		"time"
		"github.com/stretchr/testify/assert"

)

func TestSuccessfulConnection(t *testing.T) {
	fmt.Println("test success")
	startup, shutdown, _ := startServer(6590, t)
	conn, err := Start("127.0.0.1:6590",
		certPath("server/server-cert.pem"),
		certPath("client/creds/01-cert.pem"),
		certPath("client/creds/01-key.pem"))
	if err != nil {
		t.Fatalf("error in start client %v", err)
	}
	defer conn.Close()
	c := <- startup
	fmt.Println("conn")
	c.Write([]byte{0,0})
	fmt.Println("write")
	shutdown <- true
}

func TestUnsuccessfulConnection(t *testing.T) {
	fmt.Println("test unsuccessful")
	conn, err := Start("127.0.0.1:6590",
		certPath("server/server-cert.pem"),
		certPath("client/creds/01-cert.pem"),
		certPath("client/creds/01-key.pem"))
	if err != nil {
		t.Fatalf("error in start client %v", err)
	}
	defer conn.Close()
	time.Sleep(time.Millisecond * 100)
	assert.False(t, conn.IsConnected())
	fmt.Println("starting server")
	startup, shutdown, _ := startServer(6590, t)
	fmt.Println("server started")
	<- startup
	fmt.Println("startup achieved")
	time.Sleep(time.Millisecond * 10)
	assert.True(t, conn.WaitConnect())
	shutdown <- true
}

type tester struct {
	A string
	B int
	C bool
}

func TestConnectAndSend(t *testing.T) {
	startup, shutdown, recv := startServer(6590, t)
	conn, err := Start("127.0.0.1:6590",
		certPath("server/server-cert.pem"),
		certPath("client/creds/01-cert.pem"),
		certPath("client/creds/01-key.pem"))
	if err != nil {
		t.Fatalf("error in start client %v", err)
	}
	defer conn.Close()
	<- startup
	toBeSent := tester{"test", 15, true}
	conn.Send(toBeSent)
	gotBack := <- recv
	assert.Equal(t, "{\"A\":\"test\",\"B\":15,\"C\":true}", gotBack)
	shutdown <- true
}

func TestConnectAndRecv(t *testing.T) {
	startup, shutdown, _ := startServer(6590, t)
	conn, err := Start("127.0.0.1:6590",
		certPath("server/server-cert.pem"),
		certPath("client/creds/01-cert.pem"),
		certPath("client/creds/01-key.pem"))
	if err != nil {
		t.Fatalf("error in start client %v", err)
	}
	defer conn.Close()
	c := <- startup
	str := "{\"A\":\"test1\",\"B\":10,\"C\":false}"
	c.Write([]byte {0,byte(len(str))})
	c.Write([]byte(str))
	m := conn.Recv().(map[string]interface{})
	assert.Equal(t, "test1", m["A"])
	assert.Equal(t, 10, m["B"])
	assert.Equal(t, false, m["C"])
	shutdown <- true
}

func startServer(port int, t *testing.T) (<-chan net.Conn, chan<- bool, <-chan []byte) {
	config, err := loadConfig(
		certPath("client/client-cert.pem"), 
		certPath("server/creds/01-cert.pem"), 
		certPath("server/creds/01-key.pem"))
	if err != nil {
		t.Fatalf("error in config %v", err)
	}
	ln, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), config)
	if err != nil {
		t.Fatalf("error in listen %v", err)
	}
	startup := make(chan net.Conn)
	shutdown := make(chan bool)
	recv := make(chan []byte)
	go startAccept(ln, startup, shutdown, recv, t)
	return startup, shutdown, recv
}

func startAccept(ln net.Listener, startup chan<- net.Conn, shutdown <-chan bool, recv chan<- []byte, t *testing.T) {
	defer ln.Close()
	conn, err := ln.Accept()
	if err != nil {
		t.Fatalf("error in accept %v", err)
	}
	go startServerConn(conn, startup, shutdown, recv, t)
}

func startServerConn(conn net.Conn, startup chan<- net.Conn, shutdown <-chan bool, recv chan<- []byte, t *testing.T) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(time.Second))
	startup <- conn
	conn.Write([]byte {0,0})
	// go func() {
		buffer, err := readFramed(conn)
		if err != nil {
			fmt.Println("error in read", err)
		} else {
			fmt.Println("recv", buffer)
			recv <- buffer
		}
	// }()
	<- shutdown
	fmt.Println("shutdown")
}

func certPath(cert string) string {
	return fmt.Sprintf("%s/certs/%s", os.Getenv("GOPATH"), cert)
}