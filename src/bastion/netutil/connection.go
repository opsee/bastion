package netutil

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"time"
)

type Connection struct {
	Conn   net.Conn
	Reader *bufio.Reader
	Server *Server
}

func (c *Connection) Write(out []byte) (int, error) {
	return c.Conn.Write(out)
}

func (c *Connection) Read(b []byte) (int, error) {
	return c.Conn.Read(b)
}

func (c *Connection) Close() error {
	return c.Conn.Close()
}

func (c *Connection) LocalAddr() net.Addr {
	return c.Conn.LocalAddr()
}

func (c *Connection) RemoteAddr() net.Addr {
	return c.Conn.RemoteAddr()
}

func (c *Connection) SetDeadline(t time.Time) error {
	return c.Conn.SetDeadline(t)
}

func (c *Connection) SetReadDeadline(t time.Time) error {
	return c.Conn.SetReadDeadline(t)
}

func (c *Connection) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}

func (c *Connection) Loop() error {
	var err error = nil
	for {
		if err = c.HandleNextRequest(); err != nil {
			break
		}
	}
	return err
}

func (c *Connection) ReadNextRequest() (*Request, error) {
	if data, _, err := c.Reader.ReadLine(); err == nil && len(data) != 0 {
		req := NewRequest("unknown")
		if jsonErr := json.Unmarshal(data, &req); jsonErr != nil {
			return req, &net.ParseError{Type: "json", Text: string(data)}
		} else {
			return req, nil
		}
	} else if len(data) == 0 || err == io.EOF {
		return nil, io.EOF
	}
	return nil, nil
}

func (c *Connection) HandleNextRequest() error {
	if req, err := c.ReadNextRequest(); err != nil {
		return err
	} else {
		go c.Server.Handler(req, c)
		return nil
	}
}
