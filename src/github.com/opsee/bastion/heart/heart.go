package heart

import (
	"bytes"
	"encoding/json"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/nsqio/go-nsq"
	"github.com/opsee/bastion/config"
	metrics "github.com/rcrowley/go-metrics"
)

const (
	Topic     = "heartbeat"
	heartRate = 60 * time.Second
)

var (
	MetricsRegistry = metrics.NewRegistry()
)

type Heart struct {
	ProcessName string
	StopChan    chan bool
	producer    *nsq.Producer
	ticker      *time.Ticker
}

type HeartBeat struct {
	Process    string                 `json:"process_name"`
	Timestamp  int64                  `json:"timestamp"`
	Metrics    map[string]interface{} `json:"metrics"`
	CustomerId string                 `json:"customer_id"`
	BastionId  string                 `json:"bastion_id"`
}

func NewHeart(nsqdHost string, name string) (*Heart, error) {
	producer, err := nsq.NewProducer(nsqdHost, nsq.NewConfig())
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

	addMetricTypes(&heartMetrics)

	return heartMetrics, nil
}

func (this *Heart) Beat() chan error {
	errChan := make(chan error)
	customerId := config.GetConfig().CustomerId
	bastionId := config.GetConfig().BastionId
	log.Debugf("Started heartbeat for %s", this.ProcessName)
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

				hbBytes, err := json.Marshal(hb)
				if err != nil {
					errChan <- err
				} else {
					if err := this.producer.Publish(Topic, hbBytes); err != nil {
						errChan <- err
					}
				}
			case <-this.StopChan:
				this.ticker.Stop()
				break BeatLoop
			}
		}
	}(customerId, bastionId)

	return errChan
}

func addMetricTypes(heartbeat *map[string]interface{}) error {
	metricTypes := make(map[string]string)

	MetricsRegistry.Each(func(name string, i interface{}) {
		switch i.(type) {
		case metrics.Counter:
			metricTypes[name] = "counter"
		case metrics.Gauge:
			metricTypes[name] = "gauge"
		case metrics.GaugeFloat64:
			metricTypes[name] = "gaugeFloat64"
		case metrics.Healthcheck:
			metricTypes[name] = "healthcheck"
		case metrics.Histogram:
			metricTypes[name] = "histogram"
		case metrics.Meter:
			metricTypes[name] = "meter"
		case metrics.Timer:
			metricTypes[name] = "timer"
		}
	})

	(*heartbeat)["metricTypes"] = metricTypes

	return nil
}
