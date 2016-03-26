package heart

import (
	"bytes"
	"encoding/json"
	"os"
	"time"

	"github.com/opsee/bastion/messaging"
	metrics "github.com/rcrowley/go-metrics"
)

const (
	Topic     = "heartbeat"
	heartRate = 15 * time.Second
)

var (
	MetricsRegistry = metrics.NewRegistry()
)

type Heart struct {
	ProcessName string
	StopChan    chan bool
	producer    messaging.Producer
	ticker      *time.Ticker
}

type HeartBeat struct {
	Process    string                 `json:"process_name"`
	Timestamp  int64                  `json:"timestamp"`
	Metrics    map[string]interface{} `json:"metrics"`
	CustomerId string                 `json:"customer_id"`
	BastionId  string                 `json:"bastion_id"`
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

	metrics.RegisterRuntimeMemStats(MetricsRegistry)

	return heart, nil
}

func Metrics() (map[string]interface{}, error) {
	// NOTE(dan)
	// runtime.ReadMemStats calls the C functions runtime·semacquire(&runtime·worldsema) and runtime·stoptheworld()
	metrics.CaptureRuntimeMemStatsOnce(MetricsRegistry)
	b := &bytes.Buffer{}
	metrics.WriteJSONOnce(MetricsRegistry, b)
	heartMetrics := make(map[string]interface{})

	if err := json.Unmarshal(b.Bytes(), &heartMetrics); err != nil {
		return nil, err
	}
	return heartMetrics, nil
}

func (this *Heart) Beat() chan error {
	errChan := make(chan error)
	customerId := os.Getenv("BASTION_ID")
	bastionId := os.Getenv("CUSTOMER_ID")
	go func(customerId string, bastionId string) {
	BeatLoop:
		for {
			select {
			case t := <-this.ticker.C:
				metrics, err := Metrics()
				if err != nil {
					errChan <- err
				}

				hb := &HeartBeat{
					Process:    this.ProcessName,
					Timestamp:  t.UTC().UnixNano(),
					Metrics:    metrics,
					CustomerId: customerId,
					BastionId:  bastionId,
				}

				if err := this.producer.Publish(hb); err != nil {
					errChan <- err
				}
			case <-this.StopChan:
				this.ticker.Stop()
				break BeatLoop
			}
		}
	}(customerId, bastionId)

	return errChan
}
