package main

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/awscan"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/messaging"
)

const (
	moduleName = "discovery"
)

var (
	producer messaging.Producer
)

func main() {
	cfg := config.GetConfig()
	var err error

	producer, err = messaging.NewCustomerProducer(cfg.CustomerId, "discovery")

	if err != nil {
		log.WithFields(log.Fields{
			"action":  "create CustomerProducer",
			"service": "discovery",
			"errstr":  err.Error(),
		}).Error("Failed to register service with etcd")

		panic(err)
	}

	for {
		disco := awscan.NewDiscoverer(
			awscan.NewScanner(
				&awscan.Config{
					AccessKeyId: cfg.AccessKeyId,
					SecretKey:   cfg.SecretKey,
					Region:      cfg.MetaData.Region,
				},
			),
		)

		log.WithFields(log.Fields{
			"action":  "scan",
			"service": "discovery",
		}).Info("Scanning")

		for event := range disco.Discover() {
			if event.Err != nil {
				log.WithFields(log.Fields{
					"action":  "scan",
					"service": "discovery",
					"errstr":  event.Err.Error(),
				}).Error("Discovery Error")
			} else {
				err = producer.Publish(event.Result)
				if err != nil {
					log.WithFields(log.Fields{
						"action":  "scan",
						"service": "discovery",
						"errstr":  event.Err.Error(),
					}).Error("Failed to publish discovery event")
				}
			}
		}

		log.WithFields(log.Fields{
			"action":  "sleep",
			"service": "discovery",
		}).Info("Sleeping")

		time.Sleep(120 * time.Second)
	}
}
