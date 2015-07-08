package workers

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/opsee/bastion/logging"
)

type HTTPRequest struct {
	Method  string            `json:"method"`
	Target  string            `json:"target"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type HTTPResponse struct {
	Code    int               `json:"code"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
	Metrics []Metric          `json:"metrics"`
}

var (
	logger  = logging.GetLogger("http_worker")
	client  *http.Client
	dialer  *InstrumentedDialer
	metrics chan Metric
)

// Arbitrary buffer length is arbitrary.
// XXX: Revisit this when we have a better idea of messaging patterns and
// and number of goroutines, etc. Right now this exists solely so that writing
// to the metrics channel from the dialer is non-blocking, otherwise we'll be
// artificially increasing latency of the http request as a whole.
const (
	metricBufferLength int = 32
)

type Metric struct {
	Name  string                 `json:"name"`
	Value interface{}            `json:"value"`
	Tags  map[string]interface{} `json:"tags,omitempty"`
}

type InstrumentedDialer struct {
	metricChannel chan Metric
}

func NewInstrumentedDialer(mChannel chan Metric) *InstrumentedDialer {
	return &InstrumentedDialer{
		metricChannel: mChannel,
	}
}

func (d *InstrumentedDialer) MetricChannel() <-chan Metric {
	return d.metricChannel
}

func (d *InstrumentedDialer) Dial(network, addr string) (net.Conn, error) {
	ts0 := time.Now()
	ret, err := (&net.Dialer{}).Dial(network, addr)

	d.metricChannel <- Metric{
		Name:  "connect_latency_ms",
		Value: time.Since(ts0).Seconds() * 1000,
	}
	return ret, err
}

func init() {
	metrics = make(chan Metric, metricBufferLength)
	client = &http.Client{
		Transport: &http.Transport{
			Dial: NewInstrumentedDialer(metrics).Dial,
		},
	}
}

func (r *HTTPRequest) BodyReader() io.Reader {
	return strings.NewReader(r.Body)
}

func (r *HTTPRequest) Do() (*HTTPResponse, error) {
	req, err := http.NewRequest(r.Method, r.Target, r.BodyReader())
	if err != nil {
		return nil, err
	}

	for header, value := range r.Headers {
		req.Header.Add(header, value)
	}

	t0 := time.Now()
	resp, err := client.Do(req)
	metrics <- Metric{
		Name:  "request_latency_ms",
		Value: time.Since(t0).Seconds() * 1000,
	}

	defer close(metrics)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	logger.Info("Attempting to read body of response...")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	httpResponse := &HTTPResponse{
		Code:    resp.StatusCode,
		Body:    string(body),
		Metrics: []Metric{},
	}

	return httpResponse, nil
}

type HTTPWorker struct {
}

func (w *HTTPWorker) Run() {
	logger.Info("Fetching http://api-beta.opsee.co/")
	resp, err := (&HTTPRequest{
		Method: "GET",
		Target: "http://api-beta.opsee.co/health_check",
	}).Do()
	if err != nil {
		panic(err)
	}

	for metric := range metrics {
		resp.Metrics = append(resp.Metrics, metric)
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		panic(err)
	}

	fmt.Print(string(bytes))
}
