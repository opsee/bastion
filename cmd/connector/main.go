package main

import (
	"fmt"
	"net/http"
	"github.com/op/go-logging"
	"github.com/opsee/bastion"
	"github.com/opsee/bastion/aws"
	"github.com/opsee/bastion/connector"
)

var (
	log       = logging.MustGetLogger("bastion")
	logFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} [%{level:.8s}]: [%{module}] %{shortfunc} ▶ %{id:03x}%{color:reset} %{message}")
)

func main() {
	config := bastion.GetConfig()
	fmt.Println("config", config)
	httpClient := &http.Client{}
	mdp := aws.NewMetadataProvider(httpClient, config)
	connector := connector.StartConnector(config.Opsee, 1000, 1000, mdp.Get(), config)
	msg := <-connector.Recv
	fmt.Println("got", msg)
}
