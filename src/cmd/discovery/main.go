package main

import (
	"github.com/opsee/awscan"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/discovery"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/messaging"
	"time"
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

	disco := awscan.NewDiscoverer(
		awscan.NewScanner(
			&awscan.Config{
				AccessKeyId: cfg.AccessKeyId,
				SecretKey:   cfg.SecretKey,
				Region:      cfg.MetaData.Region,
			},
		),
	)
	producer, err = messaging.NewCustomerProducer(cfg.CustomerId, "discovery")

	if err != nil {
		panic(err)
	}

	for event := range disco.Discover() {
		if event.Err != nil {
			logger.Error(event.Err.Error())
		} else {
			err = producer.Publish(event.Result)
			if err != nil {
				logger.Error(err.Error())
			}
		}
	}
}
