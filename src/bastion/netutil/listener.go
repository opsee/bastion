package netutil

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"log"
	"net"
	"strconv"
	"sync/atomic"
)

type Listener struct {
	AcceptorCount   int
	ConnectionCount int32
	ListenPort      int
	Handler         RequestHandler
	ListenerChannel chan string
	Cert            tls.Certificate
	SslOptions      map[string]string
	Started         bool
	// private
	netListener net.Listener
}

func DefaultTcpListener(handler RequestHandler) *Listener {
	return NewTcpListener(1, 5666, handler, make(map[string]string))
}

func NewTcpListener(acceptorCount int, port int, handler RequestHandler, sslOptions map[string]string) *Listener {
	listener := &Listener{}
	listener.Started = false
	listener.AcceptorCount = acceptorCount
	listener.ListenPort = port
	listener.Handler = handler
	listener.ListenerChannel = make(chan string)
	listener.SslOptions = sslOptions
	return listener
}

func (listener *Listener) Serve() {
	if listener.Started {
		return
	}
	var err error
	if len(listener.SslOptions) == 0 {
		listener.netListener, err = net.Listen("tcp", ":"+strconv.Itoa(listener.ListenPort))
	} else {
		listener.Cert, err = tls.LoadX509KeyPair(listener.SslOptions["pem"], listener.SslOptions["key"])
		config := tls.Config{Certificates: []tls.Certificate{listener.Cert}}
		config.Rand = rand.Reader
		listener.netListener, err = tls.Listen("tcp", ":"+strconv.Itoa(listener.ListenPort), &config)
	}

	if err != nil {
		log.Println("[ERROR] TCP Listener didn't start: ", err)
		return
	}

	for i := 0; i < listener.AcceptorCount; i++ {
		go listener.Loop()
	}
}

func (listener *Listener) Loop() error {
	listener.Started = true
	for {
		if innerConnection, err := listener.netListener.Accept(); err != nil {
			log.Print("[ERROR] TCP listener can't accept new connection: ", err)
			return err
		} else {
			newConnection := &Connection{Conn: *innerConnection.(*net.TCPConn), Quit: make(chan bool), Reader: bufio.NewReader(NewNormaliser(innerConnection)), Listener: listener}
			listener.IncrementConnectionCount()
			go newConnection.Loop()
		}
	}
	return nil
}

func (l *Listener) IncrementConnectionCount() int32 {
	return atomic.AddInt32(&l.ConnectionCount, 1)
}

func (l *Listener) DecrementConnectionCount() int32 {
	return atomic.AddInt32(&l.ConnectionCount, -1)
}

func (l *Listener) GetConnectionCount() int32 {
	return atomic.LoadInt32(&l.ConnectionCount)
}
