package main

import (
	"fmt"
	"net/http"

	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/connector"
	"github.com/opsee/bastion/logging"
)

var (
	log = logging.GetLogger("bastion")
)

type Command struct {
	Action string `json:"action"`
	Parameters map[string]nterface{} `json:"parameters"`
}

func main() {
	configuration := config.GetConfig()
	fmt.Println("config", configuration)
	httpClient := &http.Client{}
	mdp := config.NewMetadataProvider(httpClient, configuration)
	connector := connector.StartConnector(configuration.Opsee, 1000, 1000, mdp.Get(), configuration)
	msg := <-connector.Recv
	fmt.Println("registration acknowledged", msg)
	go processCommands(connector)
}

func processCommands(connector *connector.Connector) {
	for event := range connector.Recv {

	}
}