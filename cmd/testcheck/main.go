package main

import (
	"fmt"
	"os"
	"time"

	"github.com/opsee/bastion/checker"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func main() {
	addr := os.Args[1]
	clientConn, err := grpc.Dial(addr)
	if err != nil {
		panic(err)
	}

	cc := checker.NewCheckerClient(clientConn)

	check := &checker.HttpCheck{
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

	testCheckRequest := &checker.TestCheckRequest{
		MaxHosts: 1,
		Deadline: &checker.Timestamp{
			Seconds: time.Now().Add(time.Duration(30) * time.Second).Unix(),
		},
		Check: &checker.Check{
			Target: &checker.Target{
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
