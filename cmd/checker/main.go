package main

import (
	"crypto/tls"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	log "github.com/Sirupsen/logrus"
	"github.com/nsqio/go-nsq"
	opsee "github.com/opsee/basic/service"
	"github.com/opsee/bastion/checker"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/portmapper"
)

const (
	moduleName = "checker"
)

var (
	adminPort      int
	signalsChannel = make(chan os.Signal, 1)
)

func init() {
	signal.Notify(signalsChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

func main() {
	var err error

	cfg := config.GetConfig()
	runnerConfig := &checker.NSQRunnerConfig{}

	flag.StringVar(&runnerConfig.ConsumerQueueName, "results", "results", "Result queue name.")
	flag.StringVar(&runnerConfig.ProducerQueueName, "requests", "runner", "Requests queue name.")
	flag.StringVar(&runnerConfig.ConsumerChannelName, "channel", "runner", "Consumer channel name.")
	flag.IntVar(&runnerConfig.MaxHandlers, "max_checks", 10, "Maximum concurrently executing checks.")
	flag.IntVar(&adminPort, "admin_port", 4000, "Port for the admin server.")
	flag.Parse()

	bezosConn, err := grpc.Dial(
		config.GetConfig().BezosHost,
		grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{
				InsecureSkipVerify: false,
			}),
		),
	)
	if err != nil {
		log.Fatal(err.Error())
	}
	bezosClient := opsee.NewBezosClient(bezosConn)

	runnerConfig.ConsumerNsqdHost = cfg.NsqdHost
	runnerConfig.ProducerNsqdHost = cfg.NsqdHost
	log.WithFields(log.Fields{"service": moduleName}).Info("starting up")
	resolver := checker.NewResolver(bezosClient, config.GetConfig())
	newChecker := checker.NewChecker(resolver)
	runner, err := checker.NewRemoteRunner(runnerConfig)
	if err != nil {
		log.WithFields(log.Fields{"service": moduleName, "customerId": cfg.CustomerId, "event": "create runner", "error": "couldn't create runner"}).Fatal(err.Error())
	}
	newChecker.Runner = runner

	scheduler := checker.NewScheduler(resolver)
	newChecker.Scheduler = scheduler

	producer, err := nsq.NewProducer(cfg.NsqdHost, nsq.NewConfig())
	if err != nil {
		log.WithFields(log.Fields{"service": moduleName, "customerId": cfg.CustomerId, "event": "create create producer", "error": "couldn't create producer"}).Fatal(err.Error())
	}

	scheduler.Producer = producer
	defer newChecker.Stop()

	newChecker.Port = adminPort
	if err := newChecker.Start(); err != nil {
		log.WithFields(log.Fields{"service": moduleName, "customerId": cfg.CustomerId, "event": "start checker", "error": "couldn't start checker"}).Fatal(err.Error())
	}

	portmapper.EtcdHost = cfg.EtcdHost
	portmapper.Register(moduleName, newChecker.Port)
	defer portmapper.Unregister(moduleName, newChecker.Port)

	heart, err := heart.NewHeart(cfg.NsqdHost, moduleName)
	if err != nil {
		log.WithError(err).Fatal("Couldn't initialize heartbeat.")
	}
	beatChan := heart.Beat()

	for {
		select {
		case s := <-signalsChannel:
			switch s {
			case syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT:
				os.Exit(0)
			}
		case beatErr := <-beatChan:
			log.WithError(beatErr).Error("Heartbeat error.")
		}
	}
}
