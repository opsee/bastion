package aws_command

import (
	log "github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"testing"
)

const (
	address = ":4002"
)

func server_test(t *testing.T) {
	// TODO: write actual test
	// Set up a connection to the server.
	server := NewAWSCommander()
	server.Port = 4002

	server.Start()
	conn, err := grpc.Dial(address)
	if err != nil {
		log.Fatal("did not connect: ", err)
		t.FailNow()
	}
	defer conn.Close()
	c := NewEc2Client(conn)

	r, err := c.StartInstances(context.Background(), &StartInstancesRequest{})
	if err != nil {
		log.Fatal("Err: ", err)
	}
	log.Info("Success: ", r)
}
