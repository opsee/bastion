package netutil

import (
	"bufio"
	"fmt"
	"github.com/opsee/bastion/util"
	"net"
	"sync/atomic"
)

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

	}
}

//func (c *Connection) Start() (err error) {
//	for {
//		c.requestNum.Increment()
//		if request, err := c.readMessage(); err != nil {
//			break
//		} else if err = c.handleRequest(request); err != nil {
//			break
//		}
//		select {
//		case <-serverCtx.Done():
//			err = serverCtx.Err()
//			break
//		default:
//			continue
//		}
//	}
//	c.span.CollectMemStats()
//	log.Info(c.span.JSON())
//	return err
//}
//
//func (c *Connection) readRequest() (serverRequest *ServerRequest, err error) {
//	serverRequest = &ServerRequest{server: c.server, span: util.NewSpan(fmt.Sprintf("request-%v", c.requestNum.Load()))}
//	serverRequest.ctx = util.WithValue(serverCtx, reflect.TypeOf(serverRequest), serverRequest)
//	err = DeserializeMessage(c, serverRequest)
//	return
//}


func (c *Connection) handleRequest(request *EventMessage) (err error) {
	return nil
}

func (c *Connection) Close() error {
	atomic.StoreInt32(&c.server.exit, 1)
	return c.Conn.Close()
}

var nextConnectionId util.AtomicCounter
