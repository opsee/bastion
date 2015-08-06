package checker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/op/go-logging"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/messaging"
)

const (
	httpHeaderKey      = "header"
	httpHeaderValue    = "header value"
	httpResponseString = "OK"
	httpServerPort     = 40000
)

var (
	cfg            *config.Config
	testChecker    *Checker
	bus            *testBus
	commandEvent   *testEvent
	checkCommand   *CheckCommand
	check          Check
	resolver       *testResolver
	requestChannel chan bool
	replyChannel   chan interface{}
)

func init() {
	logging.SetLevel(logging.GetLevel("DEBUG"), "checker")

	check := Check{
		Name:     "test check",
		Interval: 15,
		Path:     "/",
		Protocol: "http",
		Port:     httpServerPort,
		Verb:     "GET",
		Target: Target{
			Name: "test target",
			Type: "sg",
			Id:   "sg",
		},
	}
	checkCommand := &CheckCommand{
		Action: "test_check",
		Check:  check,
	}
	commandJSON, err := json.Marshal(checkCommand)
	if err != nil {
		logger.Fatal("Error: %s", err.Error())
	}

	commandEvent = &testEvent{
		body: string(commandJSON),
	}

	resolver = newTestResolver("127.0.0.1")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Handling request: %s", *r)
		requestChannel <- true
		headerMap := w.Header()
		headerMap[httpHeaderKey] = []string{httpHeaderValue}
		w.WriteHeader(200)
		w.Write([]byte(httpResponseString))
	})
	errChan := make(chan error, 1)
	go func() {
		errChan <- http.ListenAndServe(fmt.Sprintf(":%d", httpServerPort), nil)
	}()
}

type testEvent struct {
	body string
}

func (e *testEvent) Ack()         { return }
func (e *testEvent) Nack()        { return }
func (e *testEvent) Type() string { return "Command" }
func (e *testEvent) Body() string { return e.body }
func (e *testEvent) Reply(msg interface{}) {
	bus.results <- msg
}

type testResolver struct {
	addr *string
}

func newTestResolver(addr string) *testResolver {
	strPtr := new(string)
	*strPtr = addr
	return &testResolver{
		addr: strPtr,
	}
}

func (t *testResolver) Resolve(tgt Target) ([]*string, error) {
	logger.Debug("Resolving target: %s", tgt)
	return []*string{t.addr}, nil
}

type testBus struct {
	commands chan messaging.EventInterface
	results  chan interface{}
}

func (t *testBus) Channel() <-chan messaging.EventInterface {
	return t.commands
}

func (t *testBus) Close() error {
	return nil
}

func (t *testBus) Publish(message interface{}) error {
	evt, err := messaging.NewEvent(message)
	if err != nil {
		return err
	}
	t.results <- evt
	return nil
}

func (t *testBus) PublishRepliable(id string, msg messaging.EventInterface) error {

	return nil
}

func TestTestHttpCheck(t *testing.T) {
	setup()
	bus.commands <- commandEvent
	select {
	case r := <-requestChannel:
		t.Log("Request received by server: %s", r)
	case <-time.After(5 * time.Second):
		t.Log("Timed out waiting for request.")
		t.Fail()
	}

	var result *TestResult = nil

	select {
	case r := <-bus.results:
		t.Log("Result received on message bus: ", r)
		result = r.(*TestResult)
	case <-time.After(5 * time.Second):
		t.Log("Timed out waiting for result on message bus.")
		t.Fail()
	}

	if result == nil {
		t.Log("Received nil result.")
		t.Fail()
	} else {
		if len(result.ResponseSet) == 0 {
			t.Log("Received empty response set in result.")
			t.Fail()
		} else {
			response, ok := result.ResponseSet[0].(*HTTPResponse)
			if ok {
				if response.Code != 200 {
					t.Log("Response had non-200 status: ", response.Code)
					t.Fail()
				}

				if response.Body != httpResponseString {
					t.Log("Received incorrect response: ", response.Body)
					t.Fail()
				}
			} else {
				t.Log("Unable to cast response: ", response)
				t.Fail()
			}
		}
	}

	teardown()
}

func setup() {
	// Reset the channel for every test, so we don't accidentally read stale
	// barbage from a previous test
	requestChannel = make(chan bool, 1)
	testChecker = NewChecker()
	bus = &testBus{
		commands: make(chan messaging.EventInterface, 1),
		results:  make(chan interface{}, 1),
	}
	testChecker.Resolver = resolver
	testChecker.Consumer = bus
	testChecker.Producer = bus
	testChecker.Start()
}

func teardown() {
	close(bus.commands)
	close(bus.results)
}
