package main

import (
	"os"

	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/opsee/bastion/aws_command"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/portmapper"
)

const (
	moduleName = "aws_command"
)

var (
	signalsChannel = make(chan os.Signal, 1)
)

func init() {
	signal.Notify(signalsChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

func main() {

	config := config.GetConfig()
	commander := aws_command.NewAWSCommander()
	commander.Port = 4002

	portmapper.EtcdHost = os.Getenv("ETCD_HOST")
	portmapper.Register(moduleName, commander.Port)
	defer portmapper.Unregister(moduleName, commander.Port)

	log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "startup"}).Info("startng up aws commander")
	if err := commander.Start(); err != nil {
		log.WithFields(log.Fields{"service": moduleName, "event": "start grpc server", "err": err}).Error("Couldn't start aws_command grpc server")
		panic(err) // systemd restart
	} else {
		log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "grpc server started"}).Info("started up aws commander")
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		log.WithError(err).Fatal("Couldn't initialize heartbeat!")
	}
	beatChan := heart.Beat()

	for {
		select {
		case s := <-signalsChannel:
			switch s {
			case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				log.Info("Received signal ", s, ". Stopping.")
				os.Exit(0)
			}
		case beatErr := <-beatChan:
			log.WithError(beatErr)
		}
	}
}
