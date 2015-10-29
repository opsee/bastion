package checker

import (
	"fmt"
	"net/http"
	"os"

	"github.com/opsee/bastion/logging"
	// "github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/suite"
)

const (
	testHTTPResponseString = "OK"
	testHTTPServerPort     = 40000
)

type TestCommonStubs struct {
}

func (t TestCommonStubs) HTTPCheck() *HttpCheck {
	return &HttpCheck{
		Name:     "test check",
		Path:     "/",
		Protocol: "http",
		Port:     testHTTPServerPort,
		Verb:     "GET",
	}
}

func (t TestCommonStubs) HTTPRequest() *HTTPRequest {
	return &HTTPRequest{
		Method: "GET",
		URL:    fmt.Sprintf("http://127.0.0.1:%d/", testHTTPServerPort),
	}
}

func (t TestCommonStubs) Check() *Check {
	return &Check{
		Id:        "stub-check-id",
		Interval:  60,
		Target:    &Target{},
		CheckSpec: &Any{},
	}
}

func (t TestCommonStubs) PassingCheck() *Check {
	check := t.Check()
	check.Target = &Target{
		Type: "sg",
		Id:   "sg",
		Name: "sg",
	}

	spec, _ := MarshalAny(t.HTTPCheck())
	check.CheckSpec = spec
	return check
}

func (t TestCommonStubs) PassingCheckMultiTarget() *Check {
	check := t.Check()
	check.Target = &Target{
		Type: "sg",
		Id:   "sg3",
		Name: "sg3",
	}

	spec, _ := MarshalAny(t.HTTPCheck())
	check.CheckSpec = spec
	return check
}

func (t TestCommonStubs) BadCheck() *Check {
	check := t.Check()
	check.Target = &Target{
		Type: "sg",
		Id:   "unknown",
		Name: "unknown",
	}
	check.CheckSpec = &Any{
		TypeUrl: "unknown",
		Value:   []byte{},
	}
	return check
}

type testResolver struct {
	Targets map[string][]*Target
}

func (t *testResolver) Resolve(tgt *Target) ([]*Target, error) {
	logger.Debug("Resolving target: %s", tgt)
	if tgt.Id == "empty" {
		return []*Target{}, nil
	}
	resolved := t.Targets[tgt.Id]
	if resolved == nil {
		return nil, fmt.Errorf("")
	}

	return resolved, nil
}

func newTestResolver() *testResolver {
	addr := "127.0.0.1"
	return &testResolver{
		Targets: map[string][]*Target{
			"sg": []*Target{
				&Target{
					Id:      "id",
					Type:    "instance",
					Name:    "id",
					Address: addr,
				},
			},
			"sg3": []*Target{
				&Target{
					Id:      "id",
					Name:    "id",
					Type:    "instance",
					Address: addr,
				},
				&Target{
					Id:      "id",
					Name:    "id",
					Type:    "instance",
					Address: addr,
				},
				&Target{
					Id:      "id",
					Name:    "id",
					Type:    "instance",
					Address: addr,
				},
			},
		},
	}
}

var testEnvReady bool = false

func setupTestEnv() {
	if testEnvReady {
		return
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "INFO"
	}
	logging.SetLevel(logLevel, "checker")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Handling request: %s", *r)
		headerMap := w.Header()
		headerMap["header"] = []string{"ok"}
		w.WriteHeader(200)
		w.Write([]byte(testHTTPResponseString))
	})
	errChan := make(chan error, 1)
	go func() {
		errChan <- http.ListenAndServe(fmt.Sprintf(":%d", testHTTPServerPort), nil)
	}()

	testEnvReady = true
}
