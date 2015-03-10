package netutil

import (
	"bufio"
	"encoding/json"
	"net"
	"time"
)

type Connection struct {
	conn   net.Conn
	reader *bufio.Reader
	server *Server
}

func NewConnection(innerConnection net.Conn, server *Server) *Connection {
	return &Connection{conn: innerConnection, reader: bufio.NewReader(innerConnection), server: server}
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

func (c *Connection) ReadLine() ([]byte, error) {
	if data, _, err := c.reader.ReadLine(); err != nil {
		return nil, err
	} else {
		return data, nil
	}
}

func (c *Connection) loop() error {
	for {
		if err := c.handleNextRequest(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Connection) readNextRequest() (*Request, error) {
	var req *Request = nil
	var err error = nil
	var data []byte
	if data, err = c.ReadLine(); err == nil && len(data) != 0 {
		req = NewRequest("unknown")
		if jsonErr := json.Unmarshal(data, &req); jsonErr != nil {
			err = &net.ParseError{Type: "json", Text: string(data)}
		}
	}
	return req, err
}

func (c *Connection) handleNextRequest() error {
	if req, err := c.readNextRequest(); err != nil {
		return err
	} else {
		go func() {
			reply, keepGoing := c.server.callbacks.RequestReceived(c, req)
			if !keepGoing {
				c.Close()
			} else {
				if data, err := json.Marshal(reply); err == nil {
					c.Write(data)
					c.Write([]byte("\r\n"))
				}
			}
		}()
	}
	return nil
}
