package main

import (
	"os"

	"os/signal"
	"syscall"

	log "github.com/Sirupsen/logrus"
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

	cfg := config.GetConfig()
	commander := aws_command.NewAWSCommander()
	commander.Port = 4002

	portmapper.EtcdHost = cfg.EtcdHost
	portmapper.Register(moduleName, commander.Port)
	defer portmapper.Unregister(moduleName, commander.Port)

	log.WithFields(log.Fields{"service": moduleName, "customerId": cfg.CustomerId, "event": "startup"}).Info("startng up aws commander")
	if err := commander.Start(); err != nil {
		log.WithFields(log.Fields{"service": moduleName, "event": "start grpc server", "err": err}).Error("Couldn't start aws_command grpc server")
		panic(err) // systemd restart
	} else {
		log.WithFields(log.Fields{"service": moduleName, "customerId": cfg.CustomerId, "event": "grpc server started"}).Info("started up aws commander")
	}

	heart, err := heart.NewHeart(cfg.NsqdHost, moduleName)
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