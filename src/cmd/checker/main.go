package main

import (
	"flag"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/nsqio/go-nsq"
	"github.com/opsee/bastion/checker"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/portmapper"
	"golang.org/x/net/context"
)

const (
	moduleName = "checker"
)

func main() {
	var err error

	runnerConfig := &checker.NSQRunnerConfig{}
	flag.StringVar(&runnerConfig.ConsumerQueueName, "results", "results", "Result queue name.")
	flag.StringVar(&runnerConfig.ProducerQueueName, "requests", "runner", "Requests queue name.")
	flag.StringVar(&runnerConfig.ConsumerChannelName, "channel", "runner", "Consumer channel name.")
	flag.IntVar(&runnerConfig.MaxHandlers, "max_checks", 10, "Maximum concurrently executing checks.")
	runnerConfig.NSQDHost = os.Getenv("NSQD_HOST")
	runnerConfig.CustomerID = os.Getenv("CUSTOMER_ID")
	config := config.GetConfig()
	log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId}).Info("starting up")

	checks := checker.NewChecker()
	runner, err := checker.NewRemoteRunner(runnerConfig)
	if err != nil {
		log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "create runner", "error": "couldn't create runner"}).Fatal(err.Error())
	}
	checks.Runner = runner
	scheduler := checker.NewScheduler()
	checks.Scheduler = scheduler

	producer, err := nsq.NewProducer(os.Getenv("NSQD_HOST"), nsq.NewConfig())

	if err != nil {
		log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "create create producer", "error": "couldn't create producer"}).Fatal(err.Error())
	}

	scheduler.Producer = producer
	defer checks.Stop()

	checks.Port = 4000
	if err := checks.Start(); err != nil {
		log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "start checker", "error": "couldn't start checker"}).Fatal(err.Error())
		log.Fatal(err.Error())
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "start heartbeat", "error": "error on beat"}).Fatal(err.Error())
		panic(err)
	}

	portmapper.EtcdHost = os.Getenv("ETCD_HOST")
	portmapper.Register(moduleName, checks.Port)
	defer portmapper.Unregister(moduleName, checks.Port)

	req, err := checks.GetExistingChecks()
	if err != nil {
		ctx := context.Background()
		resp, err := checks.CreateCheck(ctx, req)

		if err != nil {
			log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "synchronize checks", "error": err.Error()}).Warn("Couldn't synchronize checks")
		} else {
			log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "synchronize checks", "response": resp}).Info("synchronized checks")
		}
	}

	err = <-heart.Beat()

	if err != nil {
		log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "heartbeat", "error": "error on hearbeat"}).Fatal(err.Error())
		panic(err)
	}
}
