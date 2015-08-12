package checker

import (
	"bufio"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

const httpWorkerTaskType = "HTTPRequest"

// HTTPRequest and HTTPResponse leave their bodies as strings to make life
// easier for now. As soon as we move away from JSON, these should be []byte.

type HTTPRequest struct {
	Method  string    `json:"method"`
	URL     string    `json:"url"`
	Headers []*Header `json:"headers"`
	Body    string    `json:"body"`
}

var (
	// NOTE: http.Client, net.Dialer are safe for concurrent use.
	client *http.Client
)

func init() {
	client = &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
			Dial: (&net.Dialer{
				Timeout: 15 * time.Second,
			}).Dial,
		},
	}

	Recruiters[httpWorkerTaskType] = NewHTTPWorker
}

func (r *HTTPRequest) Do() (Response, error) {
	req, err := http.NewRequest(r.Method, r.URL, strings.NewReader(r.Body))
	if err != nil {
		return nil, err
	}

	for _, header := range r.Headers {
		key := header.Name
		for _, value := range header.Values {
			req.Header.Add(key, value)
		}
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
	//
	// For a breakdown of potential messaging costs, see:
	// https://docs.google.com/a/opsee.co/spreadsheets/d/14Y8DvBkJMhIQoZ11C5_GKeB7NknYyt-fHJaQixkJfKs/edit?usp=sharing

	rdr := bufio.NewReader(resp.Body)
	var contentLength int64
	if resp.ContentLength > 0 {
		contentLength = resp.ContentLength
	} else {
		contentLength = 4096
	}
	length := math.Min(4096, float64(contentLength))
	body := make([]byte, int64(length))
	rdr.Read(body)

	httpResponse := &HttpResponse{
		Code: int32(resp.StatusCode),
		Body: string(body),
		Metrics: []*Metric{
			&Metric{
				Name:    "request_latency_ms",
				Decimal: time.Since(t0).Seconds() * 1000,
			},
		},
	}

	return httpResponse, nil
}

type HTTPWorker struct {
	WorkQueue chan *Task
}

func NewHTTPWorker(workQueue chan *Task) Worker {
	return &HTTPWorker{
		WorkQueue: workQueue,
	}
}

func (w *HTTPWorker) Work() {
	for task := range w.WorkQueue {
		request, ok := task.Request.(*HTTPRequest)
		if ok {
			logger.Info("request: ", request)
			if response, err := request.Do(); err != nil {
				logger.Error("error processing request: %s", *task)
				logger.Error("error: %s", err.Error())
				task.Response <- &ErrorResponse{
					Error: err,
				}
			} else {
				logger.Info("response: ", response)
				task.Response <- response
			}
		} else {
			task.Response <- &ErrorResponse{
				Error: fmt.Errorf("Unable to process request: %s", task.Request),
			}
		}
	}
}
