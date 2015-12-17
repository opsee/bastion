package checker

import (
	"fmt"

	"google.golang.org/grpc"
)

type CheckerRpcClient struct {
	Client     CheckerClient
	connection *grpc.ClientConn
}

func NewRpcClient(host string, port int) (*CheckerRpcClient, error) {
	client := &CheckerRpcClient{}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", host, port), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	client.connection = conn
	client.Client = NewCheckerClient(conn)

	return client, nil
}

func (c *CheckerRpcClient) Close() {
	c.connection.Close()
}
