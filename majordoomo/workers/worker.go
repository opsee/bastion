/* Workers are responsible for handling incoming events, processing those
 * events, and then responding to those events. An incoming message should
 * not be acknowledged until it has been handled completely.
 */
package workers

import (
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/netutil"
)

var (
	Recruiters = make(map[string]NewWorkerFunc)
	logger     = logging.GetLogger("workers")
)

type Request interface{}
type Response interface{}

type Task struct {
	Request  Request
	Response Response
	Event    netutil.EventInterface
}

type WorkQueue chan Worker

type Worker interface {
	Work(*Task)
}

type NewWorkerFunc func(chan *Task, WorkQueue) Worker
