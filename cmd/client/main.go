package main

import (
	"bastion/netutil"
	"log"
)

type MyClient struct {
}

func (c *MyClient) SslOptions() netutil.SslOptions {
	return nil
}
func (c *MyClient) ConnectionMade(*netutil.BaseClient) bool {
	return false
}

func (c *MyClient) ConnectionLost(*netutil.BaseClient, error) {

}
func (c *MyClient) ReplyReceived(*netutil.BaseClient, *netutil.Reply) bool {
	return false
}

func main() {
	if client, err := netutil.ConnectTCP(":5666", &MyClient{}); err != nil {
		log.Printf("client  connect: ", err)
		return
	} else {
		client.SendRequest("foo", nil)
	}
}
