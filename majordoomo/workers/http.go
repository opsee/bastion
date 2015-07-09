package workers

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const httpWorkerTaskType = "HTTPRequest"

// HTTPRequest and HTTPResponse leave their bodies as strings to make life
// easier for now. As soon as we move away from JSON, these should be []byte.

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
	Metrics []Metric          `json:"metrics,omitempty"`
	Error   string            `json:"error,omitempty"`
}

var (
	// NOTE: http.Client, net.Dialer are safe for concurrent use.
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

	Recruiters[httpWorkerTaskType] = NewHTTPWorker
}

func (r *HTTPRequest) Do() (*HTTPResponse, error) {
	req, err := http.NewRequest(r.Method, r.Target, strings.NewReader(r.Body))
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
	// WARNING: You cannot do this.
	//
	// 	body, err := ioutil.ReadAll(resp.Body)
	//
	// We absolutely must limit the size of the body in the response or we will
	// end up using up too much memory. There is no telling how large the bodies
	// could be. If we need to address exceptionally large HTTP bodies, then we
	// can do that in the future.

	rdr := bufio.NewReader(resp.Body)
	body := make([]byte, 1024)
	_, err = rdr.Read(body)
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
	responses chan *Task
	done      WorkQueue
}

func NewHTTPWorker(response chan *Task, done WorkQueue) Worker {
	return &HTTPWorker{
		responses: response,
		done:      done,
	}
}

func (w HTTPWorker) Work(task *Task) {
	var (
		request  *HTTPRequest
		response *HTTPResponse
		err      error
	)

	request, ok := task.Request.(*HTTPRequest)
	if ok {
		logger.Info("request: %s", request)
		response, err = request.Do()
		if err != nil {
			response = &HTTPResponse{
				Error: err.Error(),
			}
		}
	} else {
		response = &HTTPResponse{
			Error: fmt.Sprintf("Unable to process request: %s", task.Request),
		}
	}

	task.Response = response
	logger.Info("response: %s", task.Response)
	w.responses <- task
	w.done <- w
}