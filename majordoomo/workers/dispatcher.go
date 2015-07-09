package workers

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/opsee/bastion/messaging"
	"github.com/opsee/bastion/netutil"
)

type Manager struct {
	sync.Mutex
	maxWorkers int
	worker     newWorkerFunc
	queue      WorkQueue
	results    chan *Task
	count      int // requires synchronization
}

func NewManager(results chan *Task, worker newWorkerFunc, maxWorkers int) *Manager {
	m := &Manager{
		maxWorkers: maxWorkers,
		worker:     worker,
		queue:      make(WorkQueue, maxWorkers),
		results:    results,
		count:      0,
	}
	// Ensure that at least one worker is waiting so we don't block
	// when we schedule the first task.
	m.queue <- m.worker(m.results, m.queue)
	return m
}

func (m *Manager) Schedule(task *Task) error {
	logger.Info("Scheduling task: ", *task)
	select {
	case w := <-m.queue:
		logger.Info("Scheduling to worker: ", w)
		go w.Work(task)
	case <-time.After(1 * time.Second):
		m.Lock()
		if m.count < m.maxWorkers {
			newWorker := m.worker(m.results, m.queue)
			go newWorker.Work(task)
		}
		m.Unlock()
	}
	logger.Info("Scheduled task: ", *task)
	return nil
}

func deserialize(e netutil.EventInterface) (*Task, error) {
	var buf interface{}

	switch e.Type() {
	case "HTTPRequest":
		buf = &HTTPRequest{}
	default:
		return nil, fmt.Errorf("Unable to resolve type: %s", e.Type)
	}

	task := &Task{
		Request: buf,
		Event:   e,
	}
	err := json.Unmarshal([]byte(e.Body()), buf)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (d *Dispatcher) dispatch(e netutil.EventInterface) error {
	manager := d.managers[e.Type()]
	task, err := deserialize(e)
	if err != nil {
		return err
	}

	logger.Info("Sending task to worker: ", task)
	manager.Schedule(task)
	return nil
}

type Dispatcher struct {
	resultsChannel chan *Task
	consumer       *messaging.Consumer
	producer       *messaging.Producer
	managers       map[string]*Manager
}

const (
	maxRoutinesPerWorkerType = 10
)

func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		// XXX: Figure out what to do when results can't be published. The
		// contents of this channel end up getting sent to nsqd, but if we can't
		// talk to nsqd then this channel will back up. We either need to time
		// events out in the messaging subsystem (s.t. we don't have to handle)
		// managing this buffer here, or we do something else. -greg
		resultsChannel: make(chan *Task, 100),
	}

	consumer, err := messaging.NewConsumer("checks", "worker")
	if err != nil {
		logger.Error(err.Error())
		panic(err)
	}
	d.consumer = consumer

	producer, err := messaging.NewProducer("results")
	if err != nil {
		logger.Error(err.Error())
		panic(err)
	}
	d.producer = producer

	managers := make(map[string]*Manager)
	for t, w := range WorkerTypes {
		managers[t] = NewManager(d.resultsChannel, w, maxRoutinesPerWorkerType)
	}
	d.managers = managers

	return d
}

func (d *Dispatcher) Dispatch() {
	logger.Info("Beginning dispatch loop...")
	// For now rely on shutting down the consumer and producer to stop the
	// dispatcher. We can move this to a select with a stop channel at some
	// point if there is a compelling reason.
	go func() {
		for e := range d.consumer.Channel() {
			logger.Info("Received event: %s", e)
			err := d.dispatch(e)
			if err != nil {
				logger.Error(err.Error())
			}
		}
	}()

	go func() {
		for t := range d.resultsChannel {
			response := t.Response
			logger.Info("Publishing response: %s", t.Event)
			d.producer.Publish(response)
			t.Event.Ack()
		}
	}()
}

func (d *Dispatcher) Stop() {
	d.consumer.Stop()
	d.producer.Stop()
}
