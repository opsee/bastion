package checker

import (
	"sync"

	"github.com/opsee/basic/schema"
	"golang.org/x/net/context"
)

var Recruiters = &recruiters{
	recruitersMap: make(map[string]NewWorkerFunc),
}

type recruiters struct {
	recruitersMap map[string]NewWorkerFunc
	sync.Mutex
}

func (this *recruiters) RegisterWorker(workerType string, newFun NewWorkerFunc) {
	this.Lock()
	this.recruitersMap[workerType] = newFun
	this.Unlock()
}

func (this *recruiters) Get(key string) (NewWorkerFunc, bool) {
	this.Lock()
	defer this.Unlock()
	if f, ok := this.recruitersMap[key]; ok {
		return f, ok
	}
	return nil, false
}

func (this *recruiters) Keys() []string {
	this.Lock()
	defer this.Unlock()
	keys := make([]string, 0, len(this.recruitersMap))
	for k, _ := range this.recruitersMap {
		keys = append(keys, k)
	}
	return keys
}

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
