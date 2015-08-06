package checker

const (
	MaxRoutinesPerWorkerType = 10
	MaxQueueDepth            = 10
)

type WorkerGroup struct {
	WorkQueue   chan *Task
	WorkerQueue chan *Worker
	maxWorkers  int
	workerFunc  NewWorkerFunc
}

func NewWorkerGroup(workerFunc NewWorkerFunc, maxQueueDepth int, maxWorkers int) *WorkerGroup {
	m := &WorkerGroup{
		maxWorkers: maxWorkers,
		workerFunc: workerFunc,
		WorkQueue:  make(chan *Task, maxQueueDepth),
	}
	for i := 0; i < maxWorkers; i++ {
		m := workerFunc(m.WorkQueue)
		go m.Work()
	}
	return m
}

func (d *Dispatcher) dispatch(t *Task) {
	workerGroup := d.workerGroups[t.Type]

	logger.Info("Sending task to worker: ", t)
	workerGroup.WorkQueue <- t
}

type Dispatcher struct {
	Requests     chan *Task
	workerGroups map[string]*WorkerGroup
}

func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		Requests: make(chan *Task, MaxQueueDepth),
	}

	workerGroups := make(map[string]*WorkerGroup)
	for workerType, newFunc := range Recruiters {
		workerGroups[workerType] = NewWorkerGroup(newFunc, MaxQueueDepth, MaxRoutinesPerWorkerType)
	}
	d.workerGroups = workerGroups

	return d
}

func (d *Dispatcher) Dispatch() {
	go func() {
		for task := range d.Requests {
			logger.Debug("Dispatching request: %s", *task)
			workGroup := d.workerGroups[task.Type]
			workGroup.WorkQueue <- task
		}
	}()
}
