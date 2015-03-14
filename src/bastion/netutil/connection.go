package netutil

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"
)

type Connection struct {
	id     uint64
	conn   net.Conn
	reader *bufio.Reader
	server *Server
	span   *Span
}

func NewConnection(innerConnection net.Conn, server *Server) *Connection {
	var connectionId uint64 = nextConnectionId()
	return &Connection{conn: innerConnection,
		reader: bufio.NewReader(innerConnection),
		server: server,
		span:   NewSpan(fmt.Sprintf("conn-%d-%s-%s", connectionId, innerConnection.RemoteAddr(), innerConnection.LocalAddr())),
		id:     connectionId,
	}
}

func (c *Connection) Server() *Server {
	return c.server
}

func (c *Connection) Write(out []byte) (int, error) {
	return c.conn.Write(out)
}

func (c *Connection) Read(b []byte) (int, error) {
	return c.conn.Read(b)
}

func (c *Connection) Close() error {
	return c.conn.Close()
}

func (c *Connection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *Connection) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *Connection) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *Connection) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func (c *Connection) ReadLine() ([]byte, bool, error) {
	return c.reader.ReadLine()
}

func (c *Connection) Start() error {
	var err error = io.EOF
	var reqNum uint64 = 0
	for {
		span := NewSpan(fmt.Sprintf("request-%v", reqNum))
		reqNum += 1
		var request Request
		span.Start("request")
		span.Start("deserialize")
		if err = DeserializeMessage(c, &request); err != nil {
			break
		}
		span.Finish("deserialize")
		span.Start("reply")
		span.Start("process")
		reply, keepGoing := c.server.callbacks.RequestReceived(c, &request)
		span.Finish("process")
		if reply != nil {
			reply.Id = nextMessageId()
			span.Start("serialize")
			if err = SerializeMessage(c, reply); err != nil {
				break
			}
			span.Finish("serialize")
		}
		if !keepGoing {
			break
		}
		span.Finish("reply")
		span.Finish("request")
		log.Debug(span.JSON())

	}
	c.span.CollectMemStats()
	log.Info(c.span.JSON())
	return err
}

var connectionIdCounter uint64 = 0

func nextConnectionId() uint64 {
	return atomic.AddUint64(&connectionIdCounter, 1)
}
