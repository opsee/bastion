package netutil

import (
	"bufio"
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
	net.Conn
	Address   string
	callbacks Client
}

func (c *BaseClient) SendEvent(event *EventMessage) error {
	log.Info("sendEvent: %+v", event)
	return SerializeMessage(c, event)
}

func (c *BaseClient) SendRequest(command string, data MessageData) (err error) {
	request := NewRequest(command)
	request.Id = nextMessageId()
	request.Data["id"] = request.Id
	request.Data = data
	return SerializeMessage(c, request)
}

func (c *BaseClient) ReadReply() (reply *Reply, err error) {
	bufReader := bufio.NewReader(c)
	jsonData, isPrefix, err := bufReader.ReadLine()
	if err != nil || isPrefix {
		return nil, err
	}
	reply = &Reply{}
	json.Unmarshal(jsonData, reply)
	return reply, err
}
