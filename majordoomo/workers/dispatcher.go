package workers

import (
	"encoding/json"

	"github.com/opsee/bastion/messaging"
	"github.com/opsee/bastion/netutil"
)

type eventType string

type Dispatcher struct {
	consumer *messaging.Consumer
	producer *messaging.Producer
}

var (
	workers = map[eventType]Worker{
		"HTTPRequest": NewHTTPWorker(),
	}
)

func deserialize(e *netutil.Event) (interface{}, error) {
	var buf interface{}

	switch e.Type {
	case "HTTPRequest":
		buf = &HTTPRequest{}
	}

	err := json.Unmarshal([]byte(e.Body), buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func dispatch(e *netutil.Event) error {
	worker := workers[eventType(e.Type)]
	request, err := deserialize(e)
	if err != nil {
		return err
	}

	logger.Info("Sending request to worker: ", request)
	worker.Requests() <- request.(*HTTPRequest)
	return nil
}

func (d *Dispatcher) Run() {
	for t, worker := range workers {
		logger.Info("Starting worker for ", t)
		go worker.Run()
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

	logger.Info("Beginning event loop...")
	go func() {
		for e := range consumer.Channel() {
			logger.Info("Received event: %s", e)
			err := dispatch(e)
			if err != nil {
				logger.Error(err.Error())
			}
		}
	}()
}

func (d *Dispatcher) Stop() {
	d.consumer.Stop()
	d.producer.Stop()
}
