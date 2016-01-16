package checker

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/golang/protobuf/proto"
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
		Id:         "stub-check-id",
		Interval:   60,
		Name:       "fuck off",
		Target:     &Target{},
		CheckSpec:  &Any{},
		Assertions: []*Assertion{},
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

func (t TestCommonStubs) PassingCheckInstanceTarget() *Check {
	check := t.Check()
	check.Target = &Target{
		Type: "instance",
		Id:   "instance",
		Name: "instance",
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
			"instance": []*Target{
				&Target{
					Id:      "instance",
					Name:    "instance",
					Type:    "instance",
					Address: addr,
				},
			},
		},
	}
}

type NsqTopic struct {
	Topic string
}
type NsqChannel struct {
	Topic   string
	Channel string
}

type resetNsqConfig struct {
	Topics   []NsqTopic
	Channels []NsqChannel
}

func resetNsq(host string, qmap resetNsqConfig) {
	makeRequest := func(u *url.URL) error {
		client := &http.Client{}
		r := &http.Request{
			Method: "POST",
			URL:    u,
		}
		logger.Info("Making request to NSQD: %s", r.URL)
		resp, err := client.Do(r)
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		logger.Info("Response from NSQD: Code=%d Body=%s", resp.Status, body)
		return err
	}

	emptyTopic := func(t string) error {
		u, _ := url.Parse(fmt.Sprintf("http://%s:4151/topic/empty", host))
		u.RawQuery = fmt.Sprintf("topic=%s", t)
		return makeRequest(u)
	}

	emptyChannel := func(t, c string) error {
		u, _ := url.Parse(fmt.Sprintf("http://%s:4151/channel/empty", host))
		u.RawQuery = fmt.Sprintf("topic=%s&channel=%s", t, c)
		return makeRequest(u)
	}

	for _, topic := range qmap.Topics {
		if err := emptyTopic(topic.Topic); err != nil {
			panic(err)
		}
	}

	for _, channel := range qmap.Channels {
		if err := emptyChannel(channel.Topic, channel.Channel); err != nil {
			panic(err)
		}
	}
}

func setupBartnetEmulator() {
	// dead stupid bartnet api emulator with 2 hardcoded checks
	checka := &Check{
		Id:       "stub-check-id",
		Interval: 60,
		Name:     "fuck off",
		Target: &Target{
			Type: "sg",
			Id:   "sg3",
			Name: "sg3",
		},
		CheckSpec: &Any{},
	}
	checkb := &Check{
		Id:       "stub-check-id",
		Interval: 60,
		Name:     "fuck off",
		Target: &Target{
			Type: "sg",
			Id:   "sg3",
			Name: "sg3",
		},
		CheckSpec: &Any{},
	}
	checkspec := &HttpCheck{
		Name:     "test check",
		Path:     "/",
		Protocol: "http",
		Port:     testHTTPServerPort,
		Verb:     "GET",
	}
	spec, _ := MarshalAny(checkspec)

	checka.CheckSpec = spec
	checkb.CheckSpec = spec

	req := &CheckResourceRequest{
		Checks: []*Check{checka, checkb},
	}
	data, err := proto.Marshal(req)
	if err != nil {
		panic("couldn't set up bartnet endpoint emulator!")
	}

	http.HandleFunc("/checks", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, string(data), r.URL.Path)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

var testEnvReady bool = false

func setupTestEnv() {
	if testEnvReady {
		return
	}

	envLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	if envLevel == "" {
		envLevel = "error"
	}

	logLevel, err := log.ParseLevel(envLevel)
	if err != nil {
		panic(err)
	}
	log.SetLevel(logLevel)

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

	go func() {
		setupBartnetEmulator()
	}()

	testEnvReady = true
}
