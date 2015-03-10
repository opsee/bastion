package netutil

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
)

type Connection struct {
	Conn     net.TCPConn
	Quit     <-chan bool
	Reader   *bufio.Reader
	Listener *Listener
	Closed   bool
}

func (c *Connection) Send(out []byte) (int, error) {
	return c.Conn.Write(out)
}

func (c *Connection) Close() error {
	defer c.Listener.DecrementConnectionCount()
	c.Closed = true
	return c.Conn.Close()
}

func (c *Connection) ReadNextRequest() (*Request, error) {
	if data, _, err := c.Reader.ReadLine(); err == nil && len(data) != 0 {
		req := &Request{Message: make(Message)}
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

func (c *Connection) Loop() error {
	for {
		if err := c.HandleRequest(); err != nil {
			return err
		}
	}
}

func (c *Connection) HandleRequest() error {
	if req, err := c.ReadNextRequest(); err != nil {
		return err
	} else {
		go c.Listener.Handler(req, *c)
		return nil
	}
}
