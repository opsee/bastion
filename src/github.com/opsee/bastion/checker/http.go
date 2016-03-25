package checker

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/opsee/basic/schema"
)

const (
	httpWorkerTaskType = "HTTPRequest"
)

// HTTPRequest and HTTPResponse leave their bodies as strings to make life
// easier for now. As soon as we move away from JSON, these should be []byte.

type HTTPRequest struct {
	Method             string           `json:"method"`
	Host               string           `json:"host"`
	URL                string           `json:"url"`
	Headers            []*schema.Header `json:"headers"`
	Body               string           `json:"body"`
	InsecureSkipVerify bool             `json:"insecure_skip_verify"`
}

func init() {
	Recruiters[httpWorkerTaskType] = NewHTTPWorker
}

func (r *HTTPRequest) isWebSocketRequest() bool {
	url, err := url.Parse(r.URL)
	if err != nil {
		log.WithError(err).Error("Cannot parse URL")
		return false
	}

	if url.Scheme == "ws" || url.Scheme == "wss" {
		return true
	}

	for _, h := range r.Headers {
		if strings.ToLower(h.Name) == "upgrade" {
			if len(h.Values) > 0 && strings.ToLower(h.Values[0]) == "websocket" {
				return true
			}
		}
	}

	return false
}

func (r *HTTPRequest) doWebSocket() *Response {
	response := &Response{}

	tlsConfig := &tls.Config{
		ServerName:         r.Host,
		InsecureSkipVerify: r.InsecureSkipVerify,
	}

	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = tlsConfig
	dialer.HandshakeTimeout = 10 * time.Second

	t0 := time.Now()
	c, resp, err := dialer.Dial(r.URL, nil)
	if err != nil {
		log.WithError(err).Error("Failed to dial WebSocket service.")
		response.Error = err
		return response
	}

	defer c.Close()
	err = c.SetReadDeadline(time.Now().Add(BodyReadTimeout))
	if err != nil {
		log.WithError(err).Error("Failed to set read deadline on WebSocket connection.")
		response.Error = err
		return response
	}

	_, msgBytes, err := c.ReadMessage()
	if err != nil {
		log.WithError(err).Error("Error reading WebSocket message.")
		response.Error = err
	}

	httpResponse := &schema.HttpResponse{
		Code: int32(resp.StatusCode),
		Metrics: []*schema.Metric{
			&schema.Metric{
				Name:  "request_latency_ms",
				Value: time.Since(t0).Seconds() * 1000,
			},
		},
		Headers: []*schema.Header{},
	}

	if msgBytes != nil && len(msgBytes) > 0 {
		httpResponse.Body = string(msgBytes)
	}

	for k, v := range resp.Header {
		header := &schema.Header{}
		header.Name = k
		header.Values = v
		httpResponse.Headers = append(httpResponse.Headers, header)
	}

	return &Response{
		Response: httpResponse,
	}
}

func (r *HTTPRequest) Do() *Response {
	if r.isWebSocketRequest() {
		return r.doWebSocket()
	}

	tlsConfig := &tls.Config{
		ServerName:         r.Host,
		InsecureSkipVerify: r.InsecureSkipVerify,
	}

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("Not following redirect.")
		},
		Transport: &http.Transport{
			TLSClientConfig:       tlsConfig,
			ResponseHeaderTimeout: 30 * time.Second,
			Dial: (&net.Dialer{
				Timeout: 15 * time.Second,
			}).Dial,
		},
	}

	req, err := http.NewRequest(r.Method, r.URL, strings.NewReader(r.Body))
	if err != nil {
		return &Response{Error: err}
	}

	// Close the connection after we're done. It's the polite thing to do.
	req.Close = true
	// Give ourselves an out if we have to cancel the request. Close this
	// to cancel.
	cancelChannel := make(chan struct{})
	cancel := func() { close(cancelChannel) }
	req.Cancel = cancelChannel

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
	// If the http client returns a non-nil response and a non-nil
	// error, then it may be a redirect. We test.
	resp, err := client.Do(req)
	if resp == nil && err != nil {
		return &Response{Error: err}
	}

	defer resp.Body.Close()

	if resp.StatusCode < 300 && resp.StatusCode > 399 && err != nil {
		return &Response{Error: err}
	}

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

	if resp.ContentLength >= 0 && resp.ContentLength <= MaxContentLength {
		contentLength = resp.ContentLength
	} else {
		contentLength = MaxContentLength
	}

	log.WithFields(log.Fields{"Content-Length": resp.ContentLength, "contentLength": contentLength}).Debug("Setting content length.")
	body := make([]byte, int64(contentLength))
	if contentLength > 0 {
		// If the server does not close the connection and there is no Content-Length header,
		// then the HTTP Client will block indefinitely when trying to read the response body.
		// So, we have to wrap this in a timeout and cancel the request in order to continue.
		var (
			numBytes int
			err      error
		)
		done := make(chan struct{}, 1)

		go func() {
			numBytes, err = rdr.Read(body)
			done <- struct{}{}
		}()

		select {
		case <-time.After(BodyReadTimeout):
			cancel()
			err = errors.New("Timed out waiting to read body.")
		case <-done:
			// Just continue, really.
		}

		if err != nil {
			log.WithError(err).Error("Error while reading message body.")
		}

		body = bytes.Trim(body, "\x00")
		body = bytes.Trim(body, "\n")
		log.Debugf("Successfully read %i bytes...", numBytes)
	}

	httpResponse := &schema.HttpResponse{
		Code: int32(resp.StatusCode),
		Body: string(body),
		Metrics: []*schema.Metric{
			&schema.Metric{
				Name:  "request_latency_ms",
				Value: time.Since(t0).Seconds() * 1000,
			},
		},
		Headers: []*schema.Header{},
	}

	for k, v := range resp.Header {
		header := &schema.Header{}
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
