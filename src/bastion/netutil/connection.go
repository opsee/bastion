package netutil

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"time"
)

type Connection struct {
	id         int64
	conn       net.Conn
	reader     *bufio.Reader
	server     *Server
	span       *Span
	requestNum uint64
}

func NewConnection(conn net.Conn, server *Server) *Connection {
	var connectionId = nextConnectionId.Increment()
	return &Connection{conn: conn,
		reader: bufio.NewReader(conn),
		server: server,
		span:   NewSpan(fmt.Sprintf("conn-%d-%s-%s", connectionId, conn.RemoteAddr(), conn.LocalAddr())),
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

func (c *Connection) Start() (err error) {
	for {
		var request *ServerRequest
		if request, err = c.readRequest(); err != nil {
			break
		}
		if err = c.handleRequest(request); err != nil {
			break
		}
	}
	c.span.CollectMemStats()
	log.Info(c.span.JSON())
	return err
}

func (c *Connection) readRequest() (serverRequest *ServerRequest, err error) {
	serverRequest = &ServerRequest{server: c.Server(), span: NewSpan(fmt.Sprintf("request-%v", c.requestNum))}
	serverRequest.span.Start("request")
	serverRequest.span.Start("deserialize")
	err = DeserializeMessage(c, serverRequest)
	serverRequest.span.Finish("deserialize")
	return
}

func (c *Connection) handleRequest(request *ServerRequest) (err error) {
	request.span.Start("reply")
	request.span.Start("process")
	reply, keepGoing := c.server.callbacks.RequestReceived(c, request)
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

var nextConnectionId AtomicCounter
