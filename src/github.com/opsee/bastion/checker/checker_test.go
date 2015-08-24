package checker

// The Scheduler actually handles all of the check creation, so don't worry
// about testing CRUD for checks here until there's logic or some feature
// worth testing.

import (
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
)

const (
	testHTTPHeaderKey   = "header"
	testHTTPHeaderValue = "header value"
)

var (
	testChecker       *Checker
	testCheckerClient *CheckerRpcClient
	testContext       = context.TODO()

	requestStub = &TestCheckRequest{
		MaxHosts: 1,
		Deadline: &Timestamp{
			Nanos: time.Now().Add(30 * time.Minute).UnixNano(),
		},
		Check: nil,
	}
)

func setup(t *testing.T) {
	var err error

	// Reset the channel for every test, so we don't accidentally read stale
	// barbage from a previous test
	testChecker = NewChecker()
	testScheduler := NewScheduler()
	testScheduler.Resolver = newTestResolver()
	testChecker.Scheduler = testScheduler
	testChecker.Port = 4000
	testChecker.Start()
	t.Log(testChecker)

	testCheckerClient, err = NewRpcClient("127.0.0.1", 4000)
	if err != nil {
		t.Fatalf("Cannot create RPC client: %v", err)
	}
}

func teardown(t *testing.T) {
	testChecker.Stop()
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
	request, err := buildTestCheckRequest(httpCheckStub(), target)
	if err != nil {
		t.Fatalf("Unable to build test check request: target = %s, check stub = %s", target, request)
	}

	response, err := testCheckerClient.Client.TestCheck(testContext, request)
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
	request, err := buildTestCheckRequest(httpCheckStub(), target)
	if err != nil {
		t.Fatalf("Unable to build test check request: target = %s, check stub = %s", target, request)
	}

	response, err := testCheckerClient.Client.TestCheck(testContext, request)
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

func TestCheckerResolverEmpty(t *testing.T) {
	setup(t)
	target := &Target{
		Id:   "empty",
		Type: "sg",
		Name: "unknown",
	}
	request, err := buildTestCheckRequest(httpCheckStub(), target)
	if err != nil {
		t.Fatalf("Unable to build test check request: target = %s, check stub = %s", target, request)
	}

	response, err := testCheckerClient.Client.TestCheck(testContext, request)
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
	request, err := buildTestCheckRequest(httpCheckStub(), target)
	if err != nil {
		t.Fatalf("Unable to build test check request: target = %s, check stub = %s", target, request)
	}

	response, err := testCheckerClient.Client.TestCheck(testContext, request)
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
