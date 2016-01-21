package main

import (
	"fmt"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/monitor"
	"github.com/opsee/portmapper"
)

const (
	moduleName = "monitor"
)

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
	portmapper.Register(moduleName, int(cfg.AdminPort))
	defer portmapper.Unregister(moduleName, int(cfg.AdminPort))

	log.Error(http.ListenAndServe(listenAddress, nil).Error())
}
