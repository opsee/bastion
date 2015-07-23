package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/opsee/bastion/checker"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
)

const (
	moduleName = "checker"
)

var (
	logger = logging.GetLogger(moduleName)
)

func main() {
	errNum := 0
	config := config.GetConfig()

	logger.Info("Starting %s...", moduleName)
	// XXX: Holy fuck make logging easier.
	logging.SetLevel(config.LogLevel, moduleName)
	logging.SetLevel(config.LogLevel, "messaging")

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	checks := checker.NewChecker()
	checks.Resolver = checker.NewResolver(config)
	if err := checks.Start(); err != nil {
		errNum = 1
		done <- true
		logger.Error(err.Error())
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		errNum = 1
		done <- true
		logger.Error(err.Error())
	}

	go func() {
		sig := <-sigs
		logger.Info("Received %s signal, shutting down...", sig)
		checks.Stop()
		done <- true
	}()

	for {
		select {
		case <-done:
			goto Exit
		case err := <-heart.Beat():
			logger.Info("Error sending heartbeat, shutting down...")
			logger.Error(err.Error())
			errNum = 1
			goto Exit
		}
	}

Exit:
	os.Exit(errNum)
}
