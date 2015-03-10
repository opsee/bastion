package netutil

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
)

type ConnectionHandler func(*Listener, *Connection)
type RequestHandler func(*Request, *Connection)

type Listener struct {
	AcceptorCount     int
	ConnectionCount   int32
	ListenPort        int
	ConnectionHandler ConnectionHandler
	Handler           RequestHandler
	SslOptions        map[string]string
	Started           bool
	// private
	cert        tls.Certificate
	tlsConfig   *tls.Config
	netListener net.Listener
}

func DefaultTcpListener(connhandler ConnectionHandler, handler RequestHandler) *Listener {
	return NewTcpListener(1, 5666, connhandler, handler, make(map[string]string))
}

func NewTcpListener(acceptorCount int, port int, connectionHandler ConnectionHandler, requestHandler RequestHandler, sslOptions map[string]string) *Listener {
	listener := &Listener{}
	listener.Started = false
	listener.AcceptorCount = acceptorCount
	listener.ListenPort = port
	listener.ConnectionHandler = connectionHandler
	listener.Handler = requestHandler
	listener.SslOptions = sslOptions
	return listener
}

func (listener *Listener) initTLS(address string) error {
	if len(listener.SslOptions) == 0 || listener.SslOptions == nil {
		return errors.New("ssl options missing: " + fmt.Sprint(listener.SslOptions))
	}
	if cert, err := tls.LoadX509KeyPair(listener.SslOptions["pem"], listener.SslOptions["key"]); err != nil {
		return err
	} else {
		listener.cert = cert
		listener.tlsConfig = &tls.Config{Rand: rand.Reader, Certificates: []tls.Certificate{listener.cert}}
	}
	return nil
}

func (listener *Listener) initTCP(address string) error {
	return nil
}

func (listener *Listener) Serve() error {
	if listener.Started {
		return &net.AddrError{Err: "Listener already started."}
	}
	if len(listener.SslOptions) == 0 {
		if netListener, err := net.Listen("tcp", ":"+strconv.Itoa(listener.ListenPort)); err != nil {
			return err
		} else {
			listener.netListener = netListener
		}
	} else {
		// ssl config
		listener.initTLS(":" + strconv.Itoa(listener.ListenPort))
		if netListener, err := tls.Listen("tcp", ":"+strconv.Itoa(listener.ListenPort), listener.tlsConfig); err != nil {
			return err
		} else {
			listener.netListener = netListener
		}
	}
	// launch acceptor processes
	for i := 0; i < listener.AcceptorCount; i++ {
		go listener.Loop()
	}
	return nil
}

func (listener *Listener) Loop() error {
	listener.Started = true
	for {
		if innerConnection, err := listener.netListener.Accept(); err != nil {
			return err
		} else {
			listener.IncrementConnectionCount()
			newConnection := &Connection{Conn: innerConnection,
				Reader:   bufio.NewReader(innerConnection),
				Listener: listener}
			if listener.ConnectionHandler != nil {
				listener.ConnectionHandler(listener, newConnection)
			}
			go func() {
				defer listener.DecrementConnectionCount()
				for {
					if err := newConnection.HandleNextRequest(); err != nil {
						break
					}
				}
			}()
		}
	}
	return nil
}

func (listener *Listener) IncrementConnectionCount() int32 {
	return atomic.AddInt32(&listener.ConnectionCount, 1)
}

func (listener *Listener) DecrementConnectionCount() int32 {
	return atomic.AddInt32(&listener.ConnectionCount, -1)
}

func (listener *Listener) GetConnectionCount() int32 {
	return atomic.LoadInt32(&listener.ConnectionCount)
}
