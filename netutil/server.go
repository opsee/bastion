package netutil

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"github.com/opsee/bastion/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/opsee/bastion/util"
	"net"
	"os"
	"path/filepath"
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
		connectionCount util.AtomicCounter
		cert            tls.Certificate
		tlsConfig       *tls.Config
		wg              sync.WaitGroup
		exit            int32
	}

	ServerRequest struct {
		*Request
		ctx    util.Context
		server *BaseServer
		reply  *Reply
		span   *util.Span
	}
)

var (
	ErrUserCallbackClose     = errors.New("callback ordered connection closed.")
	acceptorCount        int = 4
	serverCtx            util.Context
	serverCancel         context.CancelFunc
)

func init() {
	acceptorCount = runtime.NumCPU()
}

func CancelAll() {
	serverCancel()
}

func GetFileDir() (dir string, err error) {
	dir, err = filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return dir, err
}

func NewServer(address string, handler ServerCallbacks) *BaseServer {
	serverCtx, serverCancel = util.WithCancel(util.Background())
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
		return
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
	server.wg.Wait()
}

func (server *BaseServer) initTLS() (listener net.Listener, err error) {
	if server.cert, err = tls.LoadX509KeyPair(server.SslOptions()["pem"], server.SslOptions()["key"]); err == nil {
		server.tlsConfig = &tls.Config{Rand: rand.Reader, Certificates: []tls.Certificate{server.cert}}
		listener, err = tls.Listen("tcp", server.Address, server.tlsConfig)
	}
	return nil, err
}

func (server *BaseServer) loopOnce() (err error) {
	if conn, err := server.Listener.Accept(); err == nil {
		server.handleConnection(conn)
	}
	return
}

func (server *BaseServer) loop() (err error) {
	for {
		if err = server.loopOnce(); err != nil {
			return
		}
	}
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
