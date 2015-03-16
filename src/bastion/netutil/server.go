package netutil

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
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
		net.Listener
		Serve() error
		Stop()
		Join()
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
		wg              sync.WaitGroup
		exit            int32
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
		server.wg.Add(1)
		go func() (err error) {
			defer server.wg.Done()
			if err = server.loop(); err != nil {
				log.Notice("server loop exit: %s", err.Error())
			}
			return
		}()
	}
	return
}

func (server *BaseServer) Stop() {
	atomic.StoreInt32(&server.exit, 1)
}

func (server *BaseServer) Join() {
	log.Info("waiting on wg")
	server.wg.Wait()
	log.Info("waited on wg")
}

func (server *BaseServer) initTLS() (listener net.Listener, err error) {
	if server.cert, err = tls.LoadX509KeyPair(server.SslOptions()["pem"], server.SslOptions()["key"]); err == nil {
		server.tlsConfig = &tls.Config{Rand: rand.Reader, Certificates: []tls.Certificate{server.cert}}
		listener, err = tls.Listen("tcp", server.Address, server.tlsConfig)
	}
	return nil, err
}

func (server *BaseServer) loop() (err error) {
	for server.exit == 0 {
		if conn, err := server.Listener.Accept(); err != nil {
			break
		} else {
			server.handleConnection(conn)
		}
	}
	log.Error("server exit %d", server.exit)
	return
}

func (server *BaseServer) handleConnection(conn net.Conn) {
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
