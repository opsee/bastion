package checker

import "github.com/opsee/basic/schema"

var (
	Recruiters = make(map[string]NewWorkerFunc)
)

type Request interface {
	Do() *Response
}

type Response struct {
	Response interface{}
	Error    error
}

type Task struct {
	Type     string
	Target   *schema.Target
	Request  Request
	Response *Response
}

type Worker interface {
	Work(*Task) *Task
}

type NewWorkerFunc func(chan Worker) Worker

func RegisterWorker(workerType string, newFun NewWorkerFunc) {
	Recruiters[workerType] = newFun
}
