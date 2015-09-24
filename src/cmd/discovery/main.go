package main

import (
	"fmt"
	"github.com/opsee/awscan"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/messaging"
	"sync"
	"time"
)

const (
	moduleName = "discovery"
)

var (
	logger   = logging.GetLogger(moduleName)
	producer messaging.Producer
	wg       *sync.WaitGroup
)

func main() {
	var err error

	cfg := config.GetConfig()

	scanner := awscan.NewScanner(&awscan.Config{AccessKeyId: cfg.AccessKeyId, SecretKey: cfg.SecretKey, Region: "us-west-1"})
	disco := awscan.NewDiscoverer(scanner)

	for event := range disco.Discover() {
		if event.Err != nil {
			//XXX handle aws discovery error
			fmt.Println("Error: ", event.Error.Error())
		} else {
			fmt.Println("yay: ", event.Result)
		}
	}

	wg = &sync.WaitGroup{}

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
