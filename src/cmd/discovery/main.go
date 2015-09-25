package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/opsee/awscan"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/messaging"
)

const (
	moduleName = "discovery"
)

var (
	logger                      = logging.GetLogger(moduleName)
	producer messaging.Producer = nil
)

func main() {
	var err error

	cfg := config.GetConfig()

	println(cfg.MetaData.Timestamp)
	disco := awscan.NewDiscoverer(
		awscan.NewScanner(
			&awscan.Config{
				AccessKeyId: cfg.AccessKeyId,
				SecretKey:   cfg.SecretKey,
				Region:      cfg.MetaData.Region,
			},
		),
	)

	for event := range disco.Discover() {
		if event.Err != nil {
			//XXX handle aws discovery error
			fmt.Println("Error: ", event.Err.Error())
		} else {
			fmt.Println("yay: ", event.Result)
		}
	}

	wg := &sync.WaitGroup{}

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
		// Wait for the whole scan to finish.
		wg.Wait()

		// Sleep 2 minutes between scans.
		time.Sleep(120 * time.Second)
	}

}
