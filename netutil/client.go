package netutil

import (
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


func (c *BaseClient) SendRequest(command string, data MessageData) (err error) {
	request := NewRequest(command)
	request.Id = nextMessageId()
	request.Data = data
	return SerializeMessage(c, request)
}
