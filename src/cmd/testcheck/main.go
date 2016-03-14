package main

import (
	"fmt"
	"os"
	"time"

	"github.com/opsee/basic/schema"
	opsee "github.com/opsee/basic/service"
	"github.com/opsee/bastion/checker"
	opsee_types "github.com/opsee/protobuf/opseeproto/types"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func main() {
	addr := os.Args[1]
	clientConn, err := grpc.Dial(addr)
	if err != nil {
		panic(err)
	}

	cc := opsee.NewCheckerClient(clientConn)

	check := &schema.HttpCheck{
		Name:     "test",
		Path:     "/health_check",
		Protocol: "http",
		Port:     8080,
		Verb:     "GET",
	}
	checkSpec, err := checker.MarshalAny(check)
	if err != nil {
		panic(err)
	}

	testCheckRequest := &opsee.TestCheckRequest{
		MaxHosts: 1,
		Deadline: &opsee_types.Timestamp{
			Seconds: time.Now().Add(time.Duration(30) * time.Second).Unix(),
		},
		Check: &schema.Check{
			Target: &schema.Target{
				Id:   "api-lb",
				Name: "api-lb",
				Type: "elb",
			},
			CheckSpec: checkSpec,
		},
	}

	response, err := cc.TestCheck(context.Background(), testCheckRequest)
	if err != nil {
		panic(err)
	}

	fmt.Println(response)
}
