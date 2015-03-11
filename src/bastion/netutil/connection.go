package netutil

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
	"time"
)

type Connection struct {
	conn   net.Conn
	reader *bufio.Reader
	server *Server
}

func NewConnection(innerConnection net.Conn, server *Server) *Connection {
	return &Connection{conn: innerConnection,
		reader: bufio.NewReader(innerConnection),
		server: server}
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
	if jsonData, err := json.Marshal(request); err != nil {
		return err
	} else {
		if _, err := c.Write(append(jsonData, '\r', '\n')); err != nil {
			return err
		}
	}
	return nil
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
	if data, isPrefix, err := c.ReadLine(); err == nil && len(data) != 0 {
		if isPrefix {
			log.Panic("[ERROR]: got isPrefix in *Connection.readNextRequest")
		}
		req := NewRequest("", false)
		if jsonErr := json.Unmarshal(data, &req); jsonErr != nil {
			return nil, &net.ParseError{Type: "json", Text: string(data)}
		} else {
			return req, nil
		}
	} else {
		return nil, err
	}
}

func (c *Connection) handleNextRequest() error {
	if req, err := c.readNextRequest(); err != nil {
		return err
	} else {
		go func() error {
			reply, keepGoing := c.server.callbacks.RequestReceived(c, req)
			if !keepGoing {
				c.Close()
				return io.EOF
			} else {
				if data, err := json.Marshal(reply); err != nil {
					return err
				} else {
					if _, err := c.Write(append(data, '\r', '\n')); err != nil {
						return err
					}
				}
			}
			return nil
		}()
	}
	return nil
}
