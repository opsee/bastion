package workers

import "github.com/opsee/bastion/logging"

var (
	logger = logging.GetLogger("workers")
)

type Worker interface {
	Requests() chan<- interface{}
	Responses() <-chan interface{}
	Run()
}
