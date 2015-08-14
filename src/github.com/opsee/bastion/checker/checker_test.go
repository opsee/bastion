package checker

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
	checkStub         = &HttpCheck{
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
		Target:    &Target{},
		CheckSpec: nil,
	}
)

func init() {
	logging.SetLevel(logging.GetLevel("DEBUG"), "checker")

	resolver = newTestResolver()

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

func buildTestCheckRequest(check *HttpCheck, target *Target) *TestCheckRequest {
	request := requestStub
	checkBytes, err := proto.Marshal(check)
	if err != nil {
		logger.Fatalf("Unable to marshal HttpCheck: %v", err)
	}
	checkAny := &Any{
		TypeUrl: "HttpCheck",
		Value:   checkBytes,
	}
	request.CheckSpec = checkAny
	request.Target = target
	return request
}

func TestPassingTestHttpCheck(t *testing.T) {
	setup(t)

	target := &Target{
		Id:   "sg",
		Name: "sg",
		Type: "sg",
	}
	request := buildTestCheckRequest(checkStub, target)

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

func TestResolverFailure(t *testing.T) {
	setup(t)
	target := &Target{
		Id:   "unknown",
		Type: "sg",
		Name: "unknown",
	}
	request := buildTestCheckRequest(checkStub, target)

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
	request := buildTestCheckRequest(checkStub, target)

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

/*******************************************************************************
 * CreateCheck()
 ******************************************************************************/

/*******************************************************************************
 * RetrieveCheck()
 ******************************************************************************/

/*******************************************************************************
 * DeleteCheck()
 ******************************************************************************/

/*******************************************************************************
 * UpdateCheck()
 ******************************************************************************/

func TestUpdateCheck(t *testing.T) {
	setup(t)
	_, err := testChecker.UpdateCheck(nil, nil)
	if err == nil {
		t.Fail()
	}
	teardown(t)
}

func setup(t *testing.T) {
	var err error

	// Reset the channel for every test, so we don't accidentally read stale
	// barbage from a previous test
	testChecker = NewChecker()
	testChecker.Resolver = resolver
	testChecker.Port = 4000
	testChecker.Start()

	checkerTestClient, err = NewRpcClient("127.0.0.1", 4000)
	if err != nil {
		t.Fatalf("Cannot create RPC client: %v", err)
	}
}

func teardown(t *testing.T) {
	testChecker.Stop()
}
