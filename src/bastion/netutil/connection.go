package netutil

import (
	"bufio"
	"io"
	"net"
	"time"
)

type Connection struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	server *Server
	span   *Span
}

func NewConnection(innerConnection net.Conn, server *Server) *Connection {
	span := NewSpan("connection-" + innerConnection.RemoteAddr().String() + "->" + innerConnection.LocalAddr().String())
	return &Connection{conn: innerConnection,
		reader: bufio.NewReader(innerConnection),
		writer: bufio.NewWriter(innerConnection),
		server: server,
		span:   span,
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

func (c *Connection) SendRequest(command string, data MessageData) error {
	request := NewRequest(command, true)
	request.Data = data
	return SerializeMessage(c.writer, request)
}

func (c *Connection) loop() error {
	var err error = nil
	for {
		var request Request
		if err = DeserializeMessage(c.conn, &request); err == nil {
			reply, keepGoing := c.server.callbacks.RequestReceived(c, &request)
			if reply != nil {
				if err = SerializeMessage(c.conn, reply); err != nil {
					break
				}
			}
			if !keepGoing {
				err = io.EOF
				break
			}
		}
	}
	c.server.callbacks.ConnectionLost(c, err)
	return err
}
