package checker

// The Scheduler actually handles all of the check creation, so don't worry
// about testing CRUD for checks here until there's logic or some feature
// worth testing.

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"github.com/opsee/bastion/config"
	"golang.org/x/net/context"
)

const (
	httpHeaderKey      = "header"
	httpHeaderValue    = "header value"
	httpResponseString = "OK"
	httpServerPort     = 40000
)

var (
	cfg               *config.Config
	testChecker       *Checker
	resolver          *testResolver
	checkerTestClient *CheckerRpcClient
	ctx               = context.TODO()

	httpCheckStub = &HttpCheck{
		Name:     "test check",
		Path:     "/",
		Protocol: "http",
		Port:     httpServerPort,
		Verb:     "GET",
	}

	requestStub = &TestCheckRequest{
		MaxHosts: 1,
		Deadline: &Timestamp{
			Nanos: time.Now().Add(30 * time.Minute).UnixNano(),
		},
		Check: nil,
	}
)

func init() {
	logging.SetLevel(logging.GetLevel("DEBUG"), "checker")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Handling request: %s", *r)
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

func setup(t *testing.T) {
	var err error

	// Reset the channel for every test, so we don't accidentally read stale
	// barbage from a previous test
	testChecker = NewChecker()
	scheduler := NewScheduler()
	scheduler.Resolver = newTestResolver()
	testChecker.Scheduler = scheduler
	testChecker.Port = 4000
	testChecker.Start()
	t.Log(testChecker)

	checkerTestClient, err = NewRpcClient("127.0.0.1", 4000)
	if err != nil {
		t.Fatalf("Cannot create RPC client: %v", err)
	}
}

func teardown(t *testing.T) {
	testChecker.Stop()
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

func newTestResolver() *testResolver {
	addr := "127.0.0.1"
	addrPtr := &addr
	return &testResolver{
		t: map[string]*string{
			"sg": addrPtr,
		},
	}
}

func (t *testResolver) Resolve(tgt *Target) ([]*string, error) {
	logger.Debug("Resolving target: %s", tgt)
	resolved := t.t[tgt.Id]
	if resolved == nil {
		return nil, fmt.Errorf("Unable to resolve target: %v", tgt)
	}
	return []*string{resolved}, nil
}

/*******************************************************************************
 * TestCheck()
 ******************************************************************************/

func buildTestCheckRequest(check *HttpCheck, target *Target) (*TestCheckRequest, error) {
	request := requestStub
	checkBytes, err := proto.Marshal(check)
	if err != nil {
		logger.Fatalf("Unable to marshal HttpCheck: %v", err)
		return nil, err
	}
	checkAny := &Any{
		TypeUrl: "HttpCheck",
		Value:   checkBytes,
	}

	c := testCheckStub()
	c.CheckSpec = checkAny
	c.Target = target

	request.Check = c
	return request, nil
}

func TestCheckerTestCheckRequest(t *testing.T) {
	setup(t)

	target := &Target{
		Id:   "sg",
		Name: "sg",
		Type: "sg",
	}
	request, err := buildTestCheckRequest(httpCheckStub, target)
	if err != nil {
		t.Fatalf("Unable to build test check request: target = %s, check stub = %s", target, httpCheckStub)
	}

	response, err := checkerTestClient.Client.TestCheck(ctx, request)
	if err != nil {
		t.Fatalf("Unable to get RPC response: %v", err)
	}

	httpResponse := &HttpResponse{}
	responses := response.GetResponses()

	t.Logf("Got responses: %v", responses)

	proto.Unmarshal(responses[0].Value, httpResponse)

	teardown(t)
}

func TestCheckerResolverFailure(t *testing.T) {
	setup(t)
	target := &Target{
		Id:   "unknown",
		Type: "sg",
		Name: "unknown",
	}
	request, err := buildTestCheckRequest(httpCheckStub, target)
	if err != nil {
		t.Fatalf("Unable to build test check request: target = %s, check stub = %s", target, httpCheckStub)
	}

	response, err := checkerTestClient.Client.TestCheck(ctx, request)
	if err != nil {
		t.Logf("Received error: %v", err)
	} else {
		t.Fail()
	}
	if response != nil {
		t.Fail()
	}
	teardown(t)
}

func TestTimeoutTestCheck(t *testing.T) {
	setup(t)
	target := &Target{
		Name: "test target",
		Type: "sg",
		Id:   "unknown",
	}
	request, err := buildTestCheckRequest(httpCheckStub, target)
	if err != nil {
		t.Fatalf("Unable to build test check request: target = %s, check stub = %s", target, httpCheckStub)
	}

	response, err := checkerTestClient.Client.TestCheck(ctx, request)
	if err != nil {
		t.Logf("Received error: %v", err)
	} else {
		t.Fail()
	}

	if response != nil {
		t.Fail()
	}

	teardown(t)
}

func TestUpdateCheck(t *testing.T) {
	setup(t)
	_, err := testChecker.UpdateCheck(nil, nil)
	if err == nil {
		t.Fail()
	}
	teardown(t)
}
