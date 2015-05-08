package netutil

import (
	"net"
)

type Client interface {
	SslOptions() SslOptions
	ConnectionMade(*BaseClient) bool
	ConnectionLost(*BaseClient, error)
	ReplyReceived(*BaseClient, *EventMessage) bool
}

type BaseClient struct {
	net.Conn
	Address   string
	callbacks Client
}

func (c *BaseClient) SendEvent(event *EventMessage) error {
	log.Info("sendEvent: %+v", event)
	return SerializeMessage(c, event)
}
