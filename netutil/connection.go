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
