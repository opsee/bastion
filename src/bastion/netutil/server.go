package netutil

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"runtime"
	"strconv"
	"sync/atomic"
)

type ServerCallbacks interface {
	ConnectionMade(*Connection) bool
	ConnectionLost(*Connection, error)
	RequestReceived(*Connection, *Request) (*Reply, bool)
}

type ConnectionHandler func(*Server, *Client)
type RequestHandler func(*Request, *Client)

type Server struct {
	listenPort        int
//	connectionHandler ConnectionHandler
//	handler           RequestHandler
	sslOptions        map[string]string
	acceptorCount     int
	connectionCount   int32
	cert              tls.Certificate
	tlsConfig         *tls.Config
	netListener       net.Listener
	callbacks         ServerCallbacks
}

var (
	DefaultListenPort    int = 5666
	DefaultAcceptorCount int = 4
	DefaultSSLOptions        = make(map[string]string)
)

func init() {
	DefaultAcceptorCount = runtime.NumCPU()
}

func DefaultServer(callbacks ServerCallbacks) *Server {
	return NewServer(callbacks, DefaultAcceptorCount, DefaultListenPort, DefaultSSLOptions)
}

func NewServer(callbacks ServerCallbacks, acceptorCount int, port int, sslOptions map[string]string) *Server {
	server := &Server{}
	server.acceptorCount = acceptorCount
	server.listenPort = port
	server.callbacks = callbacks
	server.sslOptions = sslOptions
	return server
}

func (server *Server) initTLS() error {
	var err error
	if server.cert, err = tls.LoadX509KeyPair(server.sslOptions["pem"], server.sslOptions["key"]); err == nil {
		server.tlsConfig = &tls.Config{Rand: rand.Reader, Certificates: []tls.Certificate{server.cert}}
		server.netListener, err = tls.Listen("tcp", ":"+strconv.Itoa(server.listenPort), server.tlsConfig)
	}
	return err
}

func (server *Server) initTCP() error {
	var err error
	server.netListener, err = net.Listen("tcp", ":"+strconv.Itoa(server.listenPort))
	return err
}

func (server *Server) Listen() error {
	if server.sslOptions != nil && len(server.sslOptions) > 0 {
		return server.initTLS()
	}
	return server.initTCP()
}

func (server *Server) Serve() error {
	if err := server.Listen(); err != nil {
		log.Print("[ERROR]: listen error: ", err)
		return err
	}
	for i := 0; i < server.acceptorCount; i++ {
		go server.loop()
	}
	return nil
}

func (server *Server) loop() error {
	for {
		if innerConnection, err := server.netListener.Accept(); err != nil {
			log.Print("[ERROR]: accept failed: ", err)
			return err
		} else {
			server.handleNewConnection(innerConnection)
		}
	}
}

func (server *Server) handleNewConnection(innerConnection net.Conn) {
	server.incrementConnectionCount()
	newConnection := NewConnection(innerConnection, server)
	if !server.callbacks.ConnectionMade(newConnection) {
		newConnection.Close()
		server.callbacks.ConnectionLost(newConnection, errors.New("callback ordered connection closed."))
	} else {
		go func() {
			var err error = nil
			defer server.decrementConnectionCount()
			err = newConnection.loop()
			server.callbacks.ConnectionLost(newConnection, err)
		}()
	}
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
