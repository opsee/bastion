package netutil

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"reflect"
	"sync/atomic"
	"github.com/opsee/bastion/util"
)

type Connection struct {
	net.Conn
	id         int64
	reader     *bufio.Reader
	server     *BaseServer
	span       *Span
	requestNum util.AtomicCounter
}

func NewConnection(conn net.Conn, server *BaseServer) *Connection {
	var connectionId = nextConnectionId.Increment()
	return &Connection{Conn: conn,
		reader: bufio.NewReader(conn),
		server: server,
		span:   NewSpan(fmt.Sprintf("conn-%d-%s-%s", connectionId, conn.RemoteAddr(), conn.LocalAddr())),
		id:     connectionId,
	}
}

func (c *Connection)  Start() (err error) {
	for {
		c.requestNum.Increment()
		if request, err := c.readRequest(); err != nil {
			break
		} else if err = c.handleRequest(request); err != nil {
			break
		}
		select {
		case <-serverCtx.Done():
			err = serverCtx.Err()
			break
		default:
			continue
		}
	}
	c.span.CollectMemStats()
	log.Info(c.span.JSON())
	return err
}

func (c *Connection) readRequest() (serverRequest *ServerRequest, err error) {
	serverRequest = &ServerRequest{server: c.server, span: NewSpan(fmt.Sprintf("request-%v", c.requestNum.Load()))}
	serverRequest.ctx = WithValue(serverCtx, reflect.TypeOf(serverRequest), serverRequest)
	err = DeserializeMessage(c, serverRequest)
	return
}

func (c *Connection) handleRequest(request *ServerRequest) (err error) {
	reply, keepGoing := c.server.RequestReceived(c, request)
	if reply != nil {
		reply.Id = nextMessageId()
		if err = SerializeMessage(c, reply); err != nil {
			return
		}
	}
	if !keepGoing {
		return io.EOF
	}
	return
}

func (c *Connection) Close() error {
	atomic.StoreInt32(&c.server.exit, 1)
	return c.Conn.Close()
}

var nextConnectionId util.AtomicCounter
