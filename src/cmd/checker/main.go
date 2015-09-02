package main

import (
	"github.com/opsee/bastion/checker"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/portmapper"
)

const (
	moduleName = "checker"
)

var (
	logger = logging.GetLogger(moduleName)
)

func main() {
	var err error

	config := config.GetConfig()

	logger.Info("Starting %s...", moduleName)
	// XXX: Holy fuck make logging easier.
	logging.SetLevel(config.LogLevel, moduleName)
	logging.SetLevel(config.LogLevel, "messaging")
	logging.SetLevel(config.LogLevel, "scanner")

	checks := checker.NewChecker()
	defer checks.Stop()

	checks.Port = 4000
	checks.Scheduler.Resolver = checker.NewResolver(config)
	if err = checks.Start(); err != nil {
		logger.Error(err.Error())
		panic(err)
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		logger.Error(err.Error())
		panic(err)
	}

	portmapper.Register(moduleName, checks.Port)
	defer portmapper.Unregister(moduleName, checks.Port)

	err = <-heart.Beat()
	panic(err)
}
