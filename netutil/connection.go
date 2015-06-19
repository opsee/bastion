package netutil

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/opsee/bastion/util"
)

type (
	SslOptions map[string]string

	ServerCallbacks interface {
		RequestReceived(*Connection, interface{}) (reply interface{}, keepGoing bool)
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
)

var (
	ErrUserCallbackClose     = errors.New("callback ordered connection closed.")
	acceptorCount        int = 4
)

func init() {
	acceptorCount = runtime.NumCPU()
}

func GetFileDir() (dir string, err error) {
	dir, err = filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return dir, err
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
		server.ConnectionLost(newConn, newConn.loop())
	}()
}

type Client interface {
	SslOptions() SslOptions
	ConnectionMade(*BaseClient) bool
	ConnectionLost(*BaseClient, error)
	ReplyReceived(*BaseClient, *EventMessage) bool
}

type BaseClient struct {
	net.Conn
	Address   string
	callbacks Client
}

func (c *BaseClient) SendEvent(event *EventMessage) error {
	log.Info("sendEvent: %+v", event)
	return SerializeMessage(c, event)
}

func (c *BaseClient) Loop() {
	for {
		msg := &EventMessage{}
		err := DeserializeMessage(c.Conn, msg)
		if err != nil {
			log.Error("error receiving message %v", err)
		}
		c.callbacks.ReplyReceived(c, msg)
	}
}

type Connection struct {
	net.Conn
	*bufio.Reader
	id         int64
	server     *BaseServer
	span       *util.Span
	requestNum util.AtomicCounter
}

func NewConnection(conn net.Conn, server *BaseServer) *Connection {
	var connectionId = nextConnectionId.Increment()
	return &Connection{Conn: conn,
		Reader: bufio.NewReader(conn),
		server: server,
		span:   util.NewSpan(fmt.Sprintf("conn-%d-%s-%s", connectionId, conn.RemoteAddr(), conn.LocalAddr())),
		id:     connectionId,
	}
}

func (c *Connection) loop() (err error) {
	for {
		c.requestNum.Increment()
		//		select {
		//		case <- serverCtx.Done():
		//			break
		//		}
	}
	c.span.CollectMemStats()
	log.Debug(c.span.JSON())
	return nil
}

func (c *Connection) readRequest() (message *EventMessage, err error) {
	return message, DeserializeMessage(c.Conn, message)
}

func (c *Connection) handleRequest(request *EventMessage) (err error) {
	return nil
}

func (c *Connection) Close() error {
	atomic.StoreInt32(&c.server.exit, 1)
	return c.Conn.Close()
}

var nextConnectionId util.AtomicCounter
