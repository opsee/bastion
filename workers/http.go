package workers

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// An HTTPRequest is a structure representing a request of the following form:
// GET http://www.google.com:80/ or generically:
// [Method] [Protocol]://[Target][Path]
// A Target must be a host and port pair delimited by a colon.
type HTTPRequest struct {
	Method   string
	Protocol string
	Target   string
	Path     string
}

// An HTTPResponse is a convenience structure that returns only the information
// needed to evaluate if the request meets assertions.
type HTTPResponse struct {
	Code    int
	Headers map[string][]string
	Body    string
	Error   error
}

type requestFunc func(url string) (*http.Response, error)

var methodMap = map[string]requestFunc{
	"GET": http.Get,
}

// RunChannel allows you to pass in a channel where requests will be
// received and returns a channel from which you can receive responses.
func RunChannel(requests <-chan HTTPRequest) <-chan HTTPResponse {
	responses := make(chan HTTPResponse, 1)
	go func() {
		defer close(responses)
		for request := range requests {
			responses <- Run(request)
		}
	}()
	return responses
}

// Run a single HTTPRequest and return a pointer to an HTTPResponse or an
// error.
func Run(req HTTPRequest) HTTPResponse {
	URLString := fmt.Sprintf("%s://%s%s", req.Protocol, req.Target, req.Path)
	var response = HTTPResponse{}

	resp, err := methodMap[req.Method](URLString)
	if err != nil {
		response.Error = err

		return response
	}

	defer resp.Body.Close()
	response.Code = resp.StatusCode
	response.Headers = resp.Header
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		response.Error = err

		return response
	}
	response.Body = string(contents)

	return response
}
