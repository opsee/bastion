package netutil

import (
	"encoding/json"
	"net"
	"time"
)

type ClientCallbacks interface {
	ConnectionMade(*Client) bool
	ConnectionLost(*Client, error)
	ReplyReceived(*Client, *Reply) bool
}

type Client struct {
	conn      *net.TCPConn
	callbacks ClientCallbacks
}

func ConnectTCP(addr string) (*Client, error) {
	if tcpAddr, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return nil, err
	} else {
		if tcpConn, err := net.DialTCP("tcp", nil, tcpAddr); err != nil {
			return nil, err
		} else {
			return &Client{conn: tcpConn, callbacks: nil}, nil
		}
	}
}

func (c *Client) SendRequest(command string, data MessageData) error {
	request := NewRequest(command, true)
	request.Data = data
	if jsonData, err := json.Marshal(request); err != nil {
		return err
	} else {
		_, err := c.Write(append(jsonData, '\r', '\n'))
        return err
	}
}

func (c *Client) Write(out []byte) (int, error) {
	return c.conn.Write(out)
}

func (c *Client) Read(b []byte) (int, error) {
	return c.conn.Read(b)
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Client) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *Client) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *Client) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *Client) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
