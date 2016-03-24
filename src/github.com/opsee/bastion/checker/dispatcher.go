package checker

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	metrics "github.com/rcrowley/go-metrics"
	"golang.org/x/net/context"
)

const (
	MaxRoutinesPerWorkerType = 10
	MaxQueueDepth            = 10
)

type workerGroup struct {
	WorkerQueue chan Worker
	workerFunc  NewWorkerFunc
}

// A TaskGroup is the unit of work for a Dispatcher.
type TaskGroup []*Task

func newWorkerGroup(workerFunc NewWorkerFunc, maxQueueDepth int, maxWorkers int) *workerGroup {
	wg := &workerGroup{
		workerFunc:  workerFunc,
		WorkerQueue: make(chan Worker, maxWorkers),
	}
	for i := 0; i < maxWorkers; i++ {
		wg.WorkerQueue <- workerFunc(wg.WorkerQueue)
	}
	return wg
}

type Dispatcher struct {
	workerGroups map[string]*workerGroup
	metrics      metrics.Registry
}

// NewDispatcher returns a dispatcher with populated internal worker groups.
func NewDispatcher() *Dispatcher {
	d := new(Dispatcher)
	d.metrics = metrics.NewPrefixedChildRegistry(metricsRegistry, "dispatcher.")

	workerGroups := make(map[string]*workerGroup)
	for _, workerType := range Recruiters.Keys() {
		if newFunc, ok := Recruiters.Get(workerType); ok {
			workerGroups[workerType] = newWorkerGroup(newFunc, MaxQueueDepth, MaxRoutinesPerWorkerType)
		} else {
			log.Warnf("Couldn't get worker type %s from Recuiters map.", workerType)
		}
	}
	d.workerGroups = workerGroups

	return d
}

// Dispatch guarantees that every Task in a TaskGroup has a response. A call to
// Dispatch will return a channel that is closed when all tasks have finished
// completion. Cancelling the context will cause Dispatch to insert an error as
// the response that indicates the context was cancelled.
func (d *Dispatcher) Dispatch(ctx context.Context, tg TaskGroup) chan *Task {
	finished := make(chan *Task, len(tg))
	defer close(finished)

	wg := &sync.WaitGroup{}

	for _, t := range tg {
		log.WithFields(log.Fields{"request": fmt.Sprintf("%#v", t.Request)}).Debug("Dispatching request.")
		metrics.GetOrRegisterCounter("task_dispatched", d.metrics).Inc(1)

		select {
		case <-ctx.Done():
			t.Response = &Response{
				Error: ctx.Err(),
			}

			log.WithFields(log.Fields{"request": fmt.Sprintf("%#v", t.Request)}).Debug("Request cancelled.")
			metrics.GetOrRegisterCounter("task_cancelled", d.metrics).Inc(1)
			finished <- t
		case w := <-d.workerGroups[t.Type].WorkerQueue:
			wg.Add(1)
			go func(worker Worker, task *Task) {
				// We rely on the worker to correctly handle context cancellation so that it
				// immediately returns once the context is cancelled.
				finished <- worker.Work(ctx, task)
				log.WithFields(log.Fields{"request": fmt.Sprintf("%#v", task.Request)}).Debug("Request finished.")
				wg.Done()
				metrics.GetOrRegisterCounter("task_executed", d.metrics).Inc(1)
			}(w, t)
		}
	}

	wg.Wait()
	log.Debug("Successfully dispatched TaskGroup. Returning.")

	return finished
}
