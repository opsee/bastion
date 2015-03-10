package netutil

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"runtime"
	"strconv"
	"sync/atomic"
)

type ServerCallbacks interface {
	ConnectionMade(*Server, *Connection)
	ConnectionLost(*Server, *Connection, error)
	RequestReceived(*Server, *Connection, *Request)
}

type ConnectionHandler func(*Server, *Connection)
type RequestHandler func(*Request, *Connection)

type Server struct {
	ListenPort        int
	ConnectionHandler ConnectionHandler
	Handler           RequestHandler
	// private
	started         bool
	sslOptions      map[string]string
	acceptorCount   int
	connectionCount int32
	cert            tls.Certificate
	tlsConfig       *tls.Config
	netListener     net.Listener
}

var (
	DefaultListenPort    int = 5666
	DefaultAcceptorCount int = 4
	DefaultSSLOptions        = make(map[string]string)
)

func init() {
	DefaultAcceptorCount = runtime.NumCPU()
}

func DefaultServer(connectionHandler ConnectionHandler, requestHandler RequestHandler) *Server {
	return NewServer(connectionHandler, requestHandler, DefaultAcceptorCount, DefaultListenPort, DefaultSSLOptions)
}

func NewServer(connectionHandler ConnectionHandler, requestHandler RequestHandler, acceptorCount int, port int, sslOptions map[string]string) *Server {
	server := &Server{}
	server.acceptorCount = acceptorCount
	server.ListenPort = port
	server.ConnectionHandler = connectionHandler
	server.Handler = requestHandler
	server.sslOptions = sslOptions
	return server
}

func (server *Server) initTLS() error {
	if len(server.sslOptions) == 0 || server.sslOptions == nil {
		return errors.New("ssl options missing: " + fmt.Sprint(server.sslOptions))
	}
	if cert, err := tls.LoadX509KeyPair(server.sslOptions["pem"], server.sslOptions["key"]); err != nil {
		return err
	} else {
		server.cert = cert
		server.tlsConfig = &tls.Config{Rand: rand.Reader, Certificates: []tls.Certificate{server.cert}}
		if netListener, err := tls.Listen("tcp", ":"+strconv.Itoa(server.ListenPort), server.tlsConfig); err != nil {
			return err
		} else {
			server.netListener = netListener
		}
	}
	return nil
}

func (server *Server) initTCP() error {
	if netListener, err := net.Listen("tcp", ":"+strconv.Itoa(server.ListenPort)); err != nil {
		return err
	} else {
		server.netListener = netListener
	}
	return nil
}

func (server *Server) Listen() error {
	if server.started {
		return errors.New("already started")
	}
	if server.sslOptions != nil && len(server.sslOptions) > 0 {
		return server.initTLS()
	}
	return server.initTCP()
}

func (server *Server) Serve() error {
	if err := server.Listen(); err != nil {
		return err
	}
	for i := 0; i < server.acceptorCount; i++ {
		go server.Loop()
	}
	server.started = true
	return nil
}

func (server *Server) Loop() error {
	for {
		if innerConnection, err := server.netListener.Accept(); err != nil {
			return err
		} else {
			server.handleNewConnection(innerConnection)
		}
	}
}

func (server *Server) handleNewConnection(innerConnection net.Conn) {
	server.incrementConnectionCount()
	newConnection := &Connection{Conn: innerConnection,
		Reader: bufio.NewReader(innerConnection),
		Server: server}
	if server.ConnectionHandler != nil {
		server.ConnectionHandler(server, newConnection)
	}
	go func() {
		defer server.decrementConnectionCount()
		if err := newConnection.Loop(); err != nil && err != io.EOF {
			log.Print("[ERROR]: Connection %p exited with error: ", err)
		}
	}()
}

func (server *Server) ConnectionCount() int32 {
	return atomic.LoadInt32(&server.connectionCount)
}

func (server *Server) incrementConnectionCount() int32 {
	return atomic.AddInt32(&server.connectionCount, 1)
}

func (server *Server) decrementConnectionCount() int32 {
	return atomic.AddInt32(&server.connectionCount, -1)
}
