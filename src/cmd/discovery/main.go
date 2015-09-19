package main

import (
	"time"

	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/messaging"
	"github.com/opsee/bastion/discovery"
)

const (
	moduleName = "discovery"
)

var (
	logger   = logging.GetLogger(moduleName)
	producer messaging.Producer
)

func main() {
	var err error

	cfg := config.GetConfig()

	producer, err = messaging.NewCustomerProducer(cfg.CustomerId, "discovery")
	if err != nil {
		panic(err)
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		panic(err)
	}
	go heart.Beat()

	
	for {
		disco := discovery.NewDiscoverer(cfg)
		for event := range disco.Discover() {
			if event.Err  != nil {
				logger.Error(event.Err.Error())
			} else {
				producer.Publish(event.Result)
			}
		}

		// Sleep 2 minutes between scans.
		time.Sleep(120 * time.Second)
	}
}
