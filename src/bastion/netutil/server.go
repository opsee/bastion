package netutil

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net"
	"runtime"
)

type (
	SslOptions map[string]string

	ServerCallbacks interface {
		RequestReceived(*Connection, *ServerRequest) (reply *Reply, keepGoing bool)
		ConnectionMade(*Connection) (keepGoing bool)
		ConnectionLost(*Connection, error)
		SslOptions() SslOptions
	}

	Listener interface {
		Serve() error
		Listen() (net.Listener, error)
	}

	TCPServer interface {
		ServerCallbacks
		Listener
	}

	BaseServer struct {
		net.Listener
		ServerCallbacks
		Address         string
		connectionCount AtomicCounter
		cert            tls.Certificate
		tlsConfig       *tls.Config
	}

	ServerRequest struct {
		*Request
		server *BaseServer
		reply  *Reply
		span   *Span
	}
)

var (
	acceptorCount        int = 4
	ErrUserCallbackClose     = errors.New("callback ordered connection closed.")
)

func init() {
	acceptorCount = runtime.NumCPU()
}

func NewServer(address string, handler ServerCallbacks) *BaseServer {

	return &BaseServer{ServerCallbacks: handler, Address: address}
}

func (server *BaseServer) Listen() (net.Listener, error) {
	if server.SslOptions() != nil && len(server.SslOptions()) > 0 {
		return server.initTLS()
	}
	return net.Listen("tcp", server.Address)
}

func (server *BaseServer) Serve() (err error) {
	if server.Listener, err = server.Listen(); err != nil {
		log.Error("listen: %v", err)
	}
	for i := 0; i < acceptorCount; i++ {
		go server.loop()
	}
	return
}

func (server *BaseServer) initTLS() (listener net.Listener, err error) {
	if server.cert, err = tls.LoadX509KeyPair(server.SslOptions()["pem"], server.SslOptions()["key"]); err == nil {
		server.tlsConfig = &tls.Config{Rand: rand.Reader, Certificates: []tls.Certificate{server.cert}}
		listener, err = tls.Listen("tcp", server.Address, server.tlsConfig)
	}
	return nil, err
}

func (server *BaseServer) loop() (err error) {
	for {
		if conn, err := server.Listener.Accept(); err != nil {
			log.Error("accept: ", err)
			break
		} else {
			server.handleNewConnection(conn)
		}
	}
	return
}

func (server *BaseServer) handleNewConnection(conn net.Conn) {
	newConn := NewConnection(conn, server)
	if !server.ConnectionMade(newConn) {
		newConn.Close()
		server.ConnectionLost(newConn, ErrUserCallbackClose)
	}
	go func() {
		server.connectionCount.Increment()
		defer server.connectionCount.Decrement()
		server.ConnectionLost(newConn, newConn.Start())
	}()
}
