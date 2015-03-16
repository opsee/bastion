package netutil

import (
	"encoding/json"
	"net"
)

type Client interface {
	SslOptions() SslOptions
	ConnectionMade(*BaseClient) bool
	ConnectionLost(*BaseClient, error)
	ReplyReceived(*BaseClient, *Reply) bool
}

type BaseClient struct {
	*net.TCPConn
	Address   string
	callbacks Client
}

func (c *BaseClient) SendRequest(command string, data MessageData) error {
	request := NewRequest(command)
	request.Id = nextMessageId()
	request.Data = data
	if jsonData, err := json.Marshal(request); err != nil {
		return err
	} else {
		_, err := c.Write(append(jsonData, '\r', '\n'))
		return err
	}
}
