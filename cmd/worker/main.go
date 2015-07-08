package main

import (
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/majordoomo/workers"
)

var (
	log = logging.GetLogger("worker")
)

func main() {
	worker := &workers.HTTPWorker{}
	worker.Run()
}
