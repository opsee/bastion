package netutil

import (
	"bufio"
	"fmt"
	"github.com/opsee/bastion/core"
	"io"
	"net"
	"reflect"
	"sync/atomic"
)

type Connection struct {
	net.Conn
	id         int64
	reader     *bufio.Reader
	server     *BaseServer
	span       *Span
	requestNum AtomicCounter
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

func (c *Connection) Server() *BaseServer {
	return c.server
}

func (c *Connection) ReadLine() ([]byte, bool, error) {
	return c.reader.ReadLine()
}

func (c *Connection) Start() (err error) {
	for {
		c.requestNum.Increment()
		//		var request *ServerRequest
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
	serverRequest = &ServerRequest{server: c.Server(), span: NewSpan(fmt.Sprintf("request-%v", c.requestNum.Load()))}
	serverRequest.ctx = core.WithValue(serverCtx, reflect.TypeOf(serverRequest), serverRequest)
	serverRequest.span.Start("request")
	serverRequest.span.Start("deserialize")
	err = DeserializeMessage(c, serverRequest)
	serverRequest.span.Finish("deserialize")
	return
}

func (c *Connection) handleRequest(request *ServerRequest) (err error) {
	request.span.Start("reply")
	request.span.Start("process")
	reply, keepGoing := c.server.RequestReceived(c, request)
	request.span.Finish("process")
	if reply != nil {
		reply.Id = nextMessageId()
		request.span.Start("serialize")
		if err = SerializeMessage(c, reply); err != nil {
			return
		}
		request.span.Finish("serialize")
	}
	if !keepGoing {
		return io.EOF
	}
	request.span.Finish("reply")
	request.span.Finish("request")
	return
}

func (c *Connection) Close() error {
	atomic.StoreInt32(&c.server.exit, 1)
	return c.Conn.Close()
}

var nextConnectionId AtomicCounter
