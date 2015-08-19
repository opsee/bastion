package main

import (
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
	var err error

	config := config.GetConfig()

	logger.Info("Starting %s...", moduleName)
	// XXX: Holy fuck make logging easier.
	logging.SetLevel(config.LogLevel, moduleName)
	logging.SetLevel(config.LogLevel, "messaging")
	logging.SetLevel(config.LogLevel, "scanner")

	checks := checker.NewChecker()
	defer checks.Stop()

	scheduler := checker.NewScheduler()
	checks.Port = 4000
	scheduler.Resolver = checker.NewResolver(config)
	if err = checks.Start(); err != nil {
		logger.Error(err.Error())
		panic(err)
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		logger.Error(err.Error())
		panic(err)
	}

	err = <-heart.Beat()
	panic(err)
}
