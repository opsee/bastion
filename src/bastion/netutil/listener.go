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

type ConnectionHandler func(*Listener, *Connection)
type RequestHandler func(*Request, Connection)

type Listener struct {
	AcceptorCount   int
	ConnectionCount int32
	ListenPort      int
    ConnectionHandler ConnectionHandler
	Handler         RequestHandler
	SslOptions      map[string]string
	Started         bool
	// private
    cert            tls.Certificate
	netListener net.Listener
}

func DefaultTcpListener(handler RequestHandler) *Listener {
	return NewTcpListener(1, 5666, nil, handler, make(map[string]string))
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



func (listener *Listener) Serve() error {
    if listener.Started {
        return &net.AddrError{Err:"Listener already started."}
    }
    if len(listener.SslOptions) == 0 {
        if netListener, err := net.Listen("tcp", ":"+strconv.Itoa(listener.ListenPort)); err != nil {
            return err
        } else {
            listener.netListener = netListener
        }
    } else { // ssl config
        if cert, err := tls.LoadX509KeyPair(listener.SslOptions["pem"], listener.SslOptions["key"]); err != nil {
            return err
        } else {
            listener.cert = cert
        }
        config := tls.Config{Certificates: []tls.Certificate{listener.cert}}
        config.Rand = rand.Reader
        if netListener, err := tls.Listen("tcp", ":"+strconv.Itoa(listener.ListenPort), &config); err != nil {
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
			log.Print("[ERROR] TCP listener can't accept new connection: ", err)
			return err
		} else {
			newConnection := &Connection{Conn: *innerConnection.(*net.TCPConn),
                Reader: bufio.NewReader(innerConnection),
                Listener: listener}
            if listener.ConnectionHandler != nil {
                listener.ConnectionHandler(listener, newConnection)
            }
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
