package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/bastion/checker"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
)

const (
	moduleName          = "runner"
	maxConcurrentChecks = 10
)

func main() {
	var err error

	runnerConfig := &checker.NSQRunnerConfig{}
	flag.StringVar(&runnerConfig.Id, "id", moduleName, "Runner identifier.")
	flag.StringVar(&runnerConfig.ProducerQueueName, "results", "results", "Result queue name.")
	flag.StringVar(&runnerConfig.ConsumerQueueName, "requests", "runner", "Requests queue name.")
	flag.StringVar(&runnerConfig.ConsumerChannelName, "channel", "runner", "Consumer channel name.")
	flag.IntVar(&runnerConfig.MaxHandlers, "max_checks", 10, "Maximum concurrently executing checks.")
	config := config.GetConfig()

	runnerConfig.NSQDHost = os.Getenv("NSQD_HOST")
	runnerConfig.CustomerID = os.Getenv("CUSTOMER_ID")

	log.Info("Starting %s...", moduleName)
	runner, err := checker.NewNSQRunner(checker.NewRunner(checker.NewResolver(config)), runnerConfig)
	if err != nil {
		log.Fatal(err.Error())
	}

	heart, err := heart.NewHeart(runnerConfig.Id)
	if err != nil {
		log.Fatal(err.Error())
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case s := <-sigs:
			log.Info("Received signal %s. Stopping...", s)
			runner.Stop()
			os.Exit(0)
		case err := <-heart.Beat():
			log.Error(err.Error())
		}
	}
}
