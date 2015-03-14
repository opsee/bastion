package netutil

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"
)

type (
	ServerCallbacks interface {
		ConnectionMade(*Connection) bool
		ConnectionLost(*Connection, error)
		RequestReceived(*Connection, *ServerRequest) (*Reply, bool)
	}

	ConnectionHandler func(*Server, *Client)
	RequestHandler    func(*ServerRequest, *Client)

	Server struct {
		listenPort      int
		sslOptions      map[string]string
		acceptorCount   int
		connectionCount AtomicCounter
		cert            tls.Certificate
		tlsConfig       *tls.Config
		netListener     net.Listener
		callbacks       ServerCallbacks
	}

	ServerRequest struct {
        *Request
		server  *Server
		reply   *Reply
		span    *Span
	}
)

var (
	DefaultListenPort    int               = 5666
	DefaultAcceptorCount int               = 4
	DefaultSSLOptions    map[string]string = make(map[string]string)
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

func (server *Server) NewRequest(request *Request) *ServerRequest {
	return &ServerRequest{Request:request, server:server, span:NewSpan(fmt.Sprintf("request-%p", request))}

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
			log.Error("[ERROR]: accept failed: ", err)
			return err
		} else {
			server.handleNewConnection(innerConnection)
			return nil
		}
	}
}

func (server *Server) handleNewConnection(innerConnection net.Conn) {
	newConnection := NewConnection(innerConnection, server)
	if !server.callbacks.ConnectionMade(newConnection) {
		newConnection.Close()
		server.callbacks.ConnectionLost(newConnection, errors.New("callback ordered connection closed."))
	} else {
		go func() {
			server.connectionCount.Increment()
			defer server.connectionCount.Decrement()
			server.callbacks.ConnectionLost(newConnection, newConnection.Start())
		}()
	}
}
