package main

import (
	"fmt"
	"net/http"

	"github.com/opsee/bastion"
	"github.com/opsee/bastion/aws"
	"github.com/opsee/bastion/connector"
	"github.com/opsee/bastion/logging"
)

var (
	log = logging.GetLogger("bastion")
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
