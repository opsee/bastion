package checker

import (
	"fmt"
	"net/http"

	"github.com/op/go-logging"
)

const (
	testHTTPResponseString = "OK"
	testHTTPServerPort     = 40000
)

func httpCheckStub() *HttpCheck {
	return &HttpCheck{
		Name:     "test check",
		Path:     "/",
		Protocol: "http",
		Port:     testHTTPServerPort,
		Verb:     "GET",
	}
}

func testCheckStub() *Check {
	return &Check{
		Id:        "string",
		Interval:  60,
		Target:    &Target{},
		CheckSpec: &Any{},
	}
}

type testResolver struct {
	t map[string]*string
}

func (t *testResolver) Resolve(tgt *Target) ([]*string, error) {
	logger.Debug("Resolving target: %s", tgt)
	if tgt.Id == "empty" {
		return []*string{}, nil
	}
	resolved := t.t[tgt.Id]
	if resolved == nil {
		return nil, fmt.Errorf("Unable to resolve target: %v", tgt)
	}
	return []*string{resolved}, nil
}

func newTestResolver() *testResolver {
	addr := "127.0.0.1"
	addrPtr := &addr
	return &testResolver{
		t: map[string]*string{
			"sg": addrPtr,
		},
	}
}

func testMakePassingTestCheck() *Check {
	check := testCheckStub()
	check.Target = &Target{
		Type: "sg",
		Id:   "sg",
		Name: "sg",
	}

	spec, _ := MarshalAny(httpCheckStub())
	check.CheckSpec = spec
	return check
}

func init() {
	logging.SetLevel(logging.GetLevel("DEBUG"), "checker")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Handling request: %s", *r)
		headerMap := w.Header()
		headerMap[testHTTPHeaderKey] = []string{testHTTPHeaderValue}
		w.WriteHeader(200)
		w.Write([]byte(testHTTPResponseString))
	})
	errChan := make(chan error, 1)
	go func() {
		errChan <- http.ListenAndServe(fmt.Sprintf(":%d", testHTTPServerPort), nil)
	}()
}
