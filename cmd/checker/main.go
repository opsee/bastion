package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/majordoomo"
)

var (
	logger = logging.GetLogger("worker")
)

func main() {
	logger.Info("Starting workers...")
	// XXX: Holy fuck make logging easier.
	if os.Getenv("DEBUG") != "" {
		logging.SetLevel("DEBUG", "worker")
		logging.SetLevel("DEBUG", "workers")
		logging.SetLevel("DEBUG", "messaging")
	}

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	dispatcher := checker.NewDispatcher()
	go dispatcher.Dispatch()

	go func() {
		sig := <-sigs
		logger.Debug("Received %s signal, shutting down...", sig)
		dispatcher.Stop()
		done <- true
	}()
	<-done
}
