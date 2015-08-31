package main

import (
	"fmt"
	"net/http"

	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/monitor"
	"github.com/opsee/bastion/registry"
)

const (
	moduleName = "monitor"
)

var (
	logger = logging.GetLogger(moduleName)
)

func main() {
	cfg := config.GetConfig()
	listenAddress := fmt.Sprintf(":%d", cfg.AdminPort)
	mon, err := monitor.NewMonitor()
	if err != nil {
		logger.Fatal(err.Error())
	}

	http.HandleFunc("/health_check", func(w http.ResponseWriter, r *http.Request) {
		if stateJSON, err := mon.SerializeState(); err != nil {
			w.WriteHeader(502)
		} else {
			w.Write(stateJSON)
		}
	})

	registry.Register(moduleName, int(cfg.AdminPort))
	defer registry.Unregister(moduleName, int(cfg.AdminPort))

	logger.Error(http.ListenAndServe(listenAddress, nil).Error())
}
