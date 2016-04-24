package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/monitor"
	"github.com/opsee/portmapper"
)

const (
	moduleName = "monitor"
)

var (
	adminPort      int
	signalsChannel = make(chan os.Signal, 1)
)

func init() {
	signal.Notify(signalsChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

func main() {
	cfg := config.GetConfig()
	flag.IntVar(&adminPort, "admin_port", 4001, "Port for the admin server.")
	flag.Parse()

	listenAddress := fmt.Sprintf(":%d", adminPort)

	mon, err := monitor.NewMonitor(config.GetConfig().NsqdHost)
	if err != nil {
		log.Fatal(err.Error())
	}

	http.HandleFunc("/health_check", func(w http.ResponseWriter, r *http.Request) {
		if stateJSON, err := mon.SerializeState(); err != nil {
			w.WriteHeader(502)
		} else {
			w.Write(stateJSON)
		}
	})

	portmapper.EtcdHost = cfg.EtcdHost
	err = portmapper.Register(moduleName, adminPort)
	if err != nil {
		log.WithError(err).Fatal("Unable to register service with portmapper.")
	}
	defer portmapper.Unregister(moduleName, adminPort)

	// serve http forever
	go func() {
		for {
			log.WithError(http.ListenAndServe(listenAddress, nil)).Fatal("Http server error. Restarting.")
		}
	}()

	heart, err := heart.NewHeart(config.GetConfig().NsqdHost, moduleName)
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
				os.Exit(0)
			}
		case beatErr := <-beatChan:
			log.WithError(beatErr).Error("Heartbeat error.")
		}
	}
}
