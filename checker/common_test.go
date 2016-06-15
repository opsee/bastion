package checker

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	log "github.com/Sirupsen/logrus"

	"github.com/gogo/protobuf/proto"
	"github.com/opsee/basic/schema"
	opsee "github.com/opsee/basic/service"
	"github.com/opsee/bastion/config"
	opsee_types "github.com/opsee/protobuf/opseeproto/types"
	"golang.org/x/net/context"
	// "github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/suite"
)

const (
	testHTTPResponseString = "OK"
	testHTTPServerPort     = 40000
)

type TestCommonStubs struct {
}

func (t TestCommonStubs) HTTPCheck() *schema.HttpCheck {
	return &schema.HttpCheck{
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

func (t TestCommonStubs) Check() *schema.Check {
	return &schema.Check{
		CustomerId: "stub-customer-id",
		Id:         "stub-check-id",
		Interval:   60,
		Name:       "fuck off",
		Target:     &schema.Target{},
		Assertions: []*schema.Assertion{},
	}
}

func (t TestCommonStubs) PassingCheck() *schema.Check {
	check := t.Check()
	check.Target = &schema.Target{
		Type: "sg",
		Id:   "sg",
		Name: "sg",
	}

	check.Spec = &schema.Check_HttpCheck{t.HTTPCheck()}
	return check
}

func (t TestCommonStubs) PassingCheckInstanceTarget() *schema.Check {
	check := t.Check()
	check.Target = &schema.Target{
		Type: "instance",
		Id:   "instance",
		Name: "instance",
	}

	check.Spec = &schema.Check_HttpCheck{t.HTTPCheck()}
	return check
}

func (t TestCommonStubs) PassingCheckMultiTarget() *schema.Check {
	check := t.Check()
	check.Target = &schema.Target{
		Type: "sg",
		Id:   "sg3",
		Name: "sg3",
	}

	check.Spec = &schema.Check_HttpCheck{t.HTTPCheck()}
	return check
}

func (t TestCommonStubs) BadCheck() *schema.Check {
	check := t.Check()
	check.Target = &schema.Target{
		Type: "sg",
		Id:   "unknown",
		Name: "unknown",
	}
	return check
}

type testResolver struct {
	Targets map[string][]*schema.Target
}

func (t *testResolver) GetUser() *schema.User {
	return &schema.User{}
}

func (t *testResolver) GetRegion() string {
	return "us-test-1"
}

func (t *testResolver) GetVpc() string {
	return "vpc-testabc00ff"
}

func (t *testResolver) Resolve(ctx context.Context, tgt *schema.Target) ([]*schema.Target, error) {
	log.Debug("Resolving target: %s", tgt)
	if tgt.Id == "empty" {
		return []*schema.Target{}, nil
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
		Targets: map[string][]*schema.Target{
			"sg": []*schema.Target{
				&schema.Target{
					Id:      "id",
					Type:    "instance",
					Name:    "id",
					Address: addr,
				},
			},
			"sg3": []*schema.Target{
				&schema.Target{
					Id:      "id",
					Name:    "id",
					Type:    "instance",
					Address: addr,
				},
				&schema.Target{
					Id:      "id",
					Name:    "id",
					Type:    "instance",
					Address: addr,
				},
				&schema.Target{
					Id:      "id",
					Name:    "id",
					Type:    "instance",
					Address: addr,
				},
			},
			"instance": []*schema.Target{
				&schema.Target{
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
		log.Info("Making request to NSQD: %s", r.URL)
		resp, err := client.Do(r)
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		log.Info("Response from NSQD: Code=%d Body=%s", resp.Status, body)
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
	checkspec := &schema.HttpCheck{
		Name:     "test check",
		Path:     "/",
		Protocol: "http",
		Port:     testHTTPServerPort,
		Verb:     "GET",
	}
	// dead stupid bartnet api emulator with 2 hardcoded checks
	checka := &schema.Check{
		CustomerId: "stub-customer-id",
		Id:         "stub-check-id",
		Interval:   60,
		Name:       "fuck off",
		Target: &schema.Target{
			Type: "sg",
			Id:   "sg3",
			Name: "sg3",
		},
		Spec: &schema.Check_HttpCheck{checkspec},
	}
	checkb := &schema.Check{
		CustomerId: "stub-customer-id",
		Id:         "stub-check-id",
		Interval:   60,
		Name:       "fuck off",
		Target: &schema.Target{
			Type: "sg",
			Id:   "sg3",
			Name: "sg3",
		},
		Spec: &schema.Check_HttpCheck{checkspec},
	}

	req := &opsee.CheckResourceRequest{
		Checks: []*schema.Check{checka, checkb},
	}
	data, err := proto.Marshal(req)
	if err != nil {
		panic("couldn't set up bartnet endpoint emulator!")
	}

	handlerPath := fmt.Sprintf("/checks/exgid/%s", config.GetConfig().CustomerId)
	http.HandleFunc(handlerPath, func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

var testEnvReady bool = false

func setupTestEnv() {
	if testEnvReady {
		return
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Debug("Handling request: %s", *r)
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
