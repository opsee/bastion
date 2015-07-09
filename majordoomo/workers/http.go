package workers

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
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
	client *http.Client
)

type Metric struct {
	Name  string                 `json:"name"`
	Value interface{}            `json:"value"`
	Tags  map[string]interface{} `json:"tags,omitempty"`
}

func init() {
	client = &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 15 * time.Second,
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
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

	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	logger.Debug("Attempting to read body of response...")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	httpResponse := &HTTPResponse{
		Code: resp.StatusCode,
		Body: string(body),
		Metrics: []Metric{
			Metric{
				Name:  "request_latency_ms",
				Value: time.Since(t0).Seconds() * 1000,
			},
		},
	}

	return httpResponse, nil
}

type HTTPWorker struct {
	requests  chan interface{}
	responses chan interface{}
}

func NewHTTPWorker() *HTTPWorker {
	return &HTTPWorker{
		requests:  make(chan interface{}, 1),
		responses: make(chan interface{}, 1),
	}
}

func (w *HTTPWorker) Requests() chan<- interface{} {
	return w.requests
}

func (w *HTTPWorker) Responses() <-chan interface{} {
	return w.responses
}

func (w *HTTPWorker) Run() {
	for r := range w.requests {
		request := r.(*HTTPRequest)
		logger.Debug("Fetching %s", request.Target)
		resp, err := request.Do()
		if err != nil {
			panic(err)
		}

		logger.Debug("Got response: %s", resp)

		w.responses <- resp
	}

	close(w.responses)
}
