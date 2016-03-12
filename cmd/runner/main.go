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

var (
	signalsChannel = make(chan os.Signal, 1)
)

func init() {
	signal.Notify(signalsChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

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
	beatChan := heart.Beat()

	for {
		select {
		case s := <-signalsChannel:
			switch s {
			case syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT:
				log.Info("Received signal ", s, ". Stopping.")
				runner.Stop()
				os.Exit(0)
			}
		case beatErr := <-beatChan:
			log.WithError(beatErr).Error("Heartbeat error.")
		}
	}
}
