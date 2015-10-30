package main

import (
	"os"
	"os/signal"
	"time"

	"github.com/opsee/bastion/heart"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/awscan"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/messaging"
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

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		log.Fatal(err.Error())
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, os.Kill)

	for {
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
		select {
		case s := <-sigs:
			log.Info("Received signal %s.  Stopping...", s)
			os.Exit(0)
		case err := <-heart.Beat():
			log.Error(err.Error())
		}
		time.Sleep(120 * time.Second)
	}
}
