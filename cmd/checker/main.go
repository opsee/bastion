package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/opsee/bastion/checker"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/logging"
)

var (
	logger = logging.GetLogger("worker")
)

func main() {
	config := config.GetConfig()

	logger.Info("Starting checker...")
	// XXX: Holy fuck make logging easier.
	logging.SetLevel(config.LogLevel, "checker")
	logging.SetLevel(config.LogLevel, "messaging")

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	checks := checker.NewChecker()
	checks.Resolver = checker.NewResolver(config)
	if err := checks.Start(); err != nil {
		done <- true
		logger.Error(err.Error())
	}

	go func() {
		sig := <-sigs
		logger.Debug("Received %s signal, shutting down...", sig)
		checks.Stop()
		done <- true
	}()

	<-done
}
