package main

import (
	"github.com/opsee/bastion/checker"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
)

const (
	moduleName = "runner"
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

	_ = checker.NewRunner(checker.NewResolver(config))

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		logger.Error(err.Error())
		panic(err)
	}

	err = <-heart.Beat()
	panic(err)
}
