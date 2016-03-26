package checker

import (
	"github.com/opsee/basic/schema"
	"golang.org/x/net/context"
)

var (
	Recruiters = make(map[string]NewWorkerFunc)
)

type Request interface {
	Do() <-chan *Response
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
	Work(context.Context, *Task) *Task
}

type NewWorkerFunc func(chan Worker) Worker

func RegisterWorker(workerType string, newFun NewWorkerFunc) {
	Recruiters[workerType] = newFun
}
