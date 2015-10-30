package main

import (
	"flag"
	"os"

	"github.com/nsqio/go-nsq"
	"github.com/opsee/bastion/checker"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/portmapper"
)

const (
	moduleName = "checker"
)

var (
	logger = logging.GetLogger(moduleName)
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

	logger.Info("Starting %s...", moduleName)
	// XXX: Holy fuck make logging easier.
	logging.SetLevel(config.LogLevel, moduleName)
	logging.SetLevel(config.LogLevel, "messaging")
	logging.SetLevel(config.LogLevel, "scanner")

	checks := checker.NewChecker()
	runner, err := checker.NewRemoteRunner(runnerConfig)
	if err != nil {
		logger.Fatal(err.Error())
	}
	checks.Runner = runner
	scheduler := checker.NewScheduler()
	checks.Scheduler = scheduler

	producer, err := nsq.NewProducer(os.Getenv("NSQD_HOST"), nsq.NewConfig())

	if err != nil {
		logger.Fatal(err)
	}

	scheduler.Producer = producer
	defer checks.Stop()

	checks.Port = 4000
	if err = checks.Start(); err != nil {
		logger.Error(err.Error())
		panic(err)
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		logger.Error(err.Error())
		panic(err)
	}

	portmapper.EtcdHost = os.Getenv("ETCD_HOST")
	portmapper.Register(moduleName, checks.Port)
	defer portmapper.Unregister(moduleName, checks.Port)

	err = <-heart.Beat()

	if err != nil {
		logger.Error(err.Error())
		panic(err)
	}
}
