package checker

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

const httpWorkerTaskType = "HTTPRequest"

// HTTPRequest and HTTPResponse leave their bodies as strings to make life
// easier for now. As soon as we move away from JSON, these should be []byte.

type HTTPRequest struct {
	Method  string    `json:"method"`
	Host    string    `json:"host"`
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
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			ResponseHeaderTimeout: 30 * time.Second,
			Dial: (&net.Dialer{
				Timeout: 15 * time.Second,
			}).Dial,
		},
	}

	Recruiters[httpWorkerTaskType] = NewHTTPWorker
}

func (r *HTTPRequest) Do() *Response {
	req, err := http.NewRequest(r.Method, r.URL, strings.NewReader(r.Body))
	if err != nil {
		return &Response{Error: err}
	}

	for _, header := range r.Headers {
		key := header.Name

		// we have to special case the host header, since the go client
		// wants that in req.Host
		if strings.ToLower(key) == "host" && len(header.Values) > 0 {
			req.Host = header.Values[0]
		}

		for _, value := range header.Values {
			req.Header.Add(key, value)
		}
	}

	// if we have set the host explicity, override any user-provided host
	if r.Host != "" {
		req.Host = r.Host
		req.Header.Set("Host", r.Host)
	}

	t0 := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return &Response{Error: err}
	}

	defer resp.Body.Close()

	log.Debug("Attempting to read body of response...")
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

	if resp.ContentLength >= 0 && resp.ContentLength < 4096 {
		contentLength = resp.ContentLength
	} else {
		contentLength = 4096
	}

	body := make([]byte, int64(contentLength))
	if contentLength > 0 {
		rdr.Read(body)
		body = bytes.Trim(body, "\x00")
		body = bytes.Trim(body, "\n")
	}

	httpResponse := &HttpResponse{
		Code: int32(resp.StatusCode),
		Body: string(body),
		Metrics: []*Metric{
			&Metric{
				Name:  "request_latency_ms",
				Value: time.Since(t0).Seconds() * 1000,
			},
		},
		Headers: []*Header{},
	}

	for k, v := range resp.Header {
		header := &Header{}
		header.Name = k
		header.Values = v
		httpResponse.Headers = append(httpResponse.Headers, header)
	}

	return &Response{
		Response: httpResponse,
	}
}

type HTTPWorker struct {
	workerQueue chan Worker
}

func NewHTTPWorker(queue chan Worker) Worker {
	return &HTTPWorker{
		workerQueue: queue,
	}
}

func (w *HTTPWorker) Work(task *Task) *Task {
	request, ok := task.Request.(*HTTPRequest)
	if ok {
		log.Debug("request: ", request)
		if response := request.Do(); response.Error != nil {
			log.Error("error processing request: %s", *task)
			log.Error("error: %s", response.Error.Error())
			task.Response = &Response{
				Error: response.Error,
			}
		} else {
			log.Debug("response: ", response)
			task.Response = response
		}
	} else {
		task.Response = &Response{
			Error: fmt.Errorf("Unable to process request: %s", task.Request),
		}
	}
	w.workerQueue <- w
	return task
}
