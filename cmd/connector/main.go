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

func main() {
	configuration := config.GetConfig()
	fmt.Println("config", configuration)
	httpClient := &http.Client{}
	mdp := config.NewMetadataProvider(httpClient, configuration)
	connector := connector.StartConnector(configuration.Opsee, 1000, 1000, mdp.Get(), configuration)
	msg := <-connector.Recv
	fmt.Println("registration acknowledged", msg)

}
