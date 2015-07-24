package main

import (
	"net/http"

	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/monitor"
)

const (
	moduleName = "monitor.main"
)

var (
	logger = logging.GetLogger(moduleName)
)

func main() {
	// _ = config.GetConfig()
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

	logger.Error(http.ListenAndServe(":4000", nil).Error())
}
