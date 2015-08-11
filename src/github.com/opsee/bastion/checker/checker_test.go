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
	testCheckRequest  *TestCheckRequest
	check             *HttpCheck
	resolver          *testResolver
	checkerTestClient *CheckerRpcClient
)

func init() {
	logging.SetLevel(logging.GetLevel("DEBUG"), "checker")

	check := &HttpCheck{
		Name:     "test check",
		Path:     "/",
		Protocol: "http",
		Port:     httpServerPort,
		Verb:     "GET",
		Target: &Target{
			Name: "test target",
			Type: "sg",
			Id:   "sg",
		},
	}

	checkBytes, err := proto.Marshal(check)
	if err != nil {
		logger.Fatalf("Unable to marshal HttpCheck: %v", err)
	}

	checkAny := &Any{
		TypeUrl: "HttpCheck",
		Value:   checkBytes,
	}

	deadline := time.Now().Add(5 * time.Second).UnixNano()
	testCheckRequest = &TestCheckRequest{
		MaxHosts: 1,
		Deadline: &Timestamp{
			Nanos: deadline,
		},
		CheckSpec: checkAny,
	}

	resolver = newTestResolver("127.0.0.1")

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

func TestTestHttpCheck(t *testing.T) {
	setup(t)

	response, err := checkerTestClient.Client.TestCheck(context.TODO(), testCheckRequest)
	if err != nil {
		t.Fatalf("Unable to get RPC response: %v", err)
	}

	httpResponse := &HttpResponse{}
	responses := response.GetResponses()

	t.Logf("Got responses: %v", responses)

	proto.Unmarshal(responses[0].Value, httpResponse)

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
