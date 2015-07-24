package heart

import (
	"runtime"
	"time"

	"github.com/opsee/bastion/messaging"
)

const (
	Topic     = "heartbeat"
	heartRate = 15 * time.Second
)

type Heart struct {
	ProcessName string
	StopChan    chan bool
	producer    messaging.Producer
	ticker      *time.Ticker
}

type HeartBeat struct {
	Process   string            `json:"process_name"`
	Timestamp int64             `json:"timestamp"`
	Metrics   map[string]uint64 `json:"metrics"`
}

func NewHeart(name string) (*Heart, error) {
	producer, err := messaging.NewProducer(Topic)
	if err != nil {
		return nil, err
	}

	heart := &Heart{
		ProcessName: name,
		producer:    producer,
		ticker:      time.NewTicker(heartRate),
	}

	return heart, nil
}

func getMetrics() map[string]uint64 {
	metrics := make(map[string]uint64)
	memStats := new(runtime.MemStats)

	metrics["num_goroutines"] = uint64(runtime.NumGoroutine())
	metrics["alloc"] = memStats.Alloc
	metrics["total_alloc"] = memStats.TotalAlloc
	metrics["sys"] = memStats.Sys
	metrics["lookups"] = memStats.Lookups
	metrics["mallocs"] = memStats.Mallocs
	metrics["frees"] = memStats.Frees
	metrics["heap_alloc"] = memStats.HeapAlloc
	metrics["heap_sys"] = memStats.HeapSys
	metrics["heap_idle"] = memStats.HeapIdle
	metrics["heap_inuse"] = memStats.HeapInuse
	metrics["heap_released"] = memStats.HeapReleased
	metrics["heap_objects"] = memStats.HeapObjects
	return metrics
}

func (h *Heart) Beat() chan error {
	errChan := make(chan error)

	go func() {
	BeatLoop:
		for {
			select {
			case t := <-h.ticker.C:
				hb := &HeartBeat{
					Process:   h.ProcessName,
					Timestamp: t.UnixNano(),
					Metrics:   getMetrics(),
				}

				if err := h.producer.Publish(hb); err != nil {
					errChan <- err
				}
			case <-h.StopChan:
				h.ticker.Stop()
				break BeatLoop
			}
		}
	}()

	return errChan
}
