package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/bastion/aws_command"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/portmapper"
	"os/signal"
	"syscall"
	"time"
)

const (
	moduleName = "aws_command"
)

var (
	signalsChannel = make(chan os.Signal, 1)
)

func main() {

	config := config.GetConfig()
	signal.Notify(signalsChannel, syscall.SIGTERM, syscall.SIGINT)
	commander := aws_command.NewAWSCommander()
	commander.Port = 4002

	portmapper.EtcdHost = os.Getenv("ETCD_HOST")
	portmapper.Register(moduleName, commander.Port)
	defer portmapper.Unregister(moduleName, commander.Port)

	go func() {
		heart, err := heart.NewHeart(moduleName)
		if err != nil {
			log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "start heartbeat", "error": "error on beat"}).Fatal(err.Error())
			panic(err)
		}

		for {
			err = <-heart.Beat()

			if err != nil {
				log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "heartbeat", "error": "error on hearbeat"}).Fatal(err.Error())
				panic(err)
			}
			time.Sleep(15 * time.Second)
		}
	}()

	log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "startup"}).Info("startng up aws commander")
	if err := commander.Start(); err != nil {
		log.WithFields(log.Fields{"service": moduleName, "event": "start grpc server", "err": err}).Error("Couldn't start aws_command grpc server")
		panic(err) // systemd restart
	} else {
		log.WithFields(log.Fields{"service": moduleName, "customerId": config.CustomerId, "event": "grpc server started"}).Info("started up aws commander")
	}

	for {
		select {
		case s := <-signalsChannel:
			switch s {
			case syscall.SIGTERM, syscall.SIGINT:
				os.Exit(1)
			}
		}
	}
}
