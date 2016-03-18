package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/monitor"
	"github.com/opsee/portmapper"
)

const (
	moduleName = "monitor"
)

var (
	signalsChannel = make(chan os.Signal, 1)
)

func init() {
	signal.Notify(signalsChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

func main() {
	cfg := config.GetConfig()
	listenAddress := fmt.Sprintf(":%d", cfg.AdminPort)

	mon, err := monitor.NewMonitor()
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

	portmapper.EtcdHost = os.Getenv("ETCD_HOST")
	err = portmapper.Register(moduleName, int(cfg.AdminPort))
	if err != nil {
		log.WithError(err).Fatal("Unable to register service with portmapper.")
	}
	defer portmapper.Unregister(moduleName, int(cfg.AdminPort))

	// serve http forever
	go func() {
		for {
			log.WithError(http.ListenAndServe(listenAddress, nil)).Fatal("Http server error. Restarting.")
		}
	}()

	for {
		select {
		case s := <-signalsChannel:
			switch s {
			case syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT:
				log.Info("Received signal ", s, ". Stopping.")
				os.Exit(0)
			}
		}
	}
}
