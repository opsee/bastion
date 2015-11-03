package main

import (
	log "github.com/Sirupsen/logrus"
	"testing"
)

// TODO write actual tests
func TestGetChecks(t *testing.T) {
	checks, err := getChecks()
	if err != nil {
		log.WithFields(log.Fields{"service": "checker", "Error": err.Error()}).Fatal("Error unmarshalling checks.")
	} else {
		log.WithFields(log.Fields{"service": "checker", "#checks": len(checks)}).Info("got some checks!")
		for c := 0; c < len(checks); c++ {
			log.Info(checks[c].Id)
		}
	}
}
