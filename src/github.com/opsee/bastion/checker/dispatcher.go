package checker

import (
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
}

// NewDispatcher returns a dispatcher with populated internal worker groups.
func NewDispatcher() *Dispatcher {
	d := new(Dispatcher)

	workerGroups := make(map[string]*workerGroup)
	for workerType, newFunc := range Recruiters {
		workerGroups[workerType] = newWorkerGroup(newFunc, MaxQueueDepth, MaxRoutinesPerWorkerType)
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
	go func() {
		defer close(finished)
		cancelled := false

		for _, t := range tg {
			select {
			case <-ctx.Done():
				cancelled = true
			default:
			}

			if !cancelled {
				logger.Debug("Dispatching request: task = %s, type = %s", *t, t.Type)
				worker := <-d.workerGroups[t.Type].WorkerQueue
				finished <- worker.Work(t)
			} else {
				t.Response = &Response{
					Error: ctx.Err(),
				}
				finished <- t
			}
		}
	}()

	return finished
}
