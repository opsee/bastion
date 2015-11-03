package checker

// The Scheduler actually handles all of the check creation, so don't worry
// about testing CRUD for checks here until there's logic or some feature
// worth testing.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

type testPublisher struct {
	MsgChan chan []byte
}

func (t *testPublisher) Publish(topic string, msg []byte) error {
	t.MsgChan <- msg
	return nil
}

func (t *testPublisher) Stop() {
	close(t.MsgChan)
}

type CheckerTestSuite struct {
	suite.Suite
	Common           TestCommonStubs
	Checker          *Checker
	CheckerClient    *CheckerRpcClient
	Context          context.Context
	TestCheckRequest *TestCheckRequest
	Publisher        Publisher
	NSQRunner        *NSQRunner
	RunnerConfig     *NSQRunnerConfig
	ResetNsqConfig   resetNsqConfig
}

func (s *CheckerTestSuite) SetupSuite() {
	cfg := &NSQRunnerConfig{
		NSQDHost:            os.Getenv("NSQD_HOST"),
		ConsumerQueueName:   "test-runner",
		ProducerQueueName:   "test-results",
		ConsumerChannelName: "test-runner",
		MaxHandlers:         1,
		CustomerID:          "test",
	}
	runner, err := NewNSQRunner(NewRunner(newTestResolver()), cfg)
	if err != nil {
		panic(err)
	}

	s.NSQRunner = runner
	s.RunnerConfig = cfg
	s.ResetNsqConfig = resetNsqConfig{
		Topics: []NsqTopic{
			NsqTopic{s.RunnerConfig.ProducerQueueName},
			NsqTopic{s.RunnerConfig.ConsumerQueueName},
		},
		Channels: []NsqChannel{
			NsqChannel{s.RunnerConfig.ProducerQueueName, "test-check-results"},
			NsqChannel{s.RunnerConfig.ConsumerQueueName, s.RunnerConfig.ConsumerChannelName},
		},
	}
}

func (s *CheckerTestSuite) SetupTest() {
	resetNsq(strings.Split(s.RunnerConfig.NSQDHost, ":")[0], s.ResetNsqConfig)
	var err error

	checker := NewChecker()
	testRunner, err := NewRemoteRunner(&NSQRunnerConfig{
		NSQDHost:            s.RunnerConfig.NSQDHost,
		ConsumerQueueName:   s.RunnerConfig.ProducerQueueName,
		ConsumerChannelName: "test-check-results",
		ProducerQueueName:   s.RunnerConfig.ConsumerQueueName,
		MaxHandlers:         1,
		CustomerID:          s.RunnerConfig.CustomerID,
	})
	if err != nil {
		panic(err)
	}

	checker.Scheduler = NewScheduler()
	checker.Scheduler.Producer = &testPublisher{make(chan []byte, 1)}
	checker.Runner = testRunner
	checker.Port = 4000
	checker.Start()

	s.Checker = checker

	checkerClient, err := NewRpcClient("127.0.0.1", 4000)
	if err != nil {
		panic(err)
	}

	s.CheckerClient = checkerClient
	s.Context = context.Background()
	s.Common = TestCommonStubs{}
	s.TestCheckRequest = &TestCheckRequest{
		MaxHosts: 1,
		Deadline: nil,
		Check:    nil,
	}

	s.Publisher = &testPublisher{}
}

func (s *CheckerTestSuite) TearDownTest() {
	s.CheckerClient.Close()
	s.Checker.Stop()
	// Reset NSQ state every time so that we don't end up reading stale garbage.
	resetNsq(strings.Split(s.RunnerConfig.NSQDHost, ":")[0], s.ResetNsqConfig)
}

/*******************************************************************************
 * SynchronizeChecks()
 ******************************************************************************/
func (s *CheckerTestSuite) TestSynchronizeChecks() {
	req, err := s.Checker.GetExistingChecks()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), req)

	resp, err := s.CheckerClient.Client.CreateCheck(s.Context, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
}

/*******************************************************************************
 * CreateCheck()
 ******************************************************************************/

func (s *CheckerTestSuite) TestGoodCreateCheckRequest() {
	req := &CheckResourceRequest{
		Checks: []*Check{s.Common.PassingCheck()},
	}
	resp, err := s.CheckerClient.Client.CreateCheck(s.Context, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), s.Common.PassingCheck().Id, resp.Responses[0].Check.Id)
}

/*******************************************************************************
 * TestCheck()
 ******************************************************************************/

func (s *CheckerTestSuite) buildTestCheckRequest(check *HttpCheck, target *Target) (*TestCheckRequest, error) {
	request := s.TestCheckRequest
	checkBytes, err := proto.Marshal(check)
	if err != nil {
		logger.Fatalf("Unable to marshal HttpCheck: %v", err)
		return nil, err
	}
	checkAny := &Any{
		TypeUrl: "HttpCheck",
		Value:   checkBytes,
	}

	c := s.Common.Check()
	c.CheckSpec = checkAny
	c.Target = target

	request.Check = c
	request.Deadline = &Timestamp{
		Nanos: time.Now().Add(3 * time.Second).UnixNano(),
	}
	return request, nil
}

func (s *CheckerTestSuite) TestCheckHasSingleResponse() {
	target := &Target{
		Id:   "sg",
		Name: "sg",
		Type: "sg",
	}
	request, err := s.buildTestCheckRequest(s.Common.HTTPCheck(), target)
	assert.NoError(s.T(), err)

	response, err := s.CheckerClient.Client.TestCheck(s.Context, request)
	assert.NoError(s.T(), err)
	assert.IsType(s.T(), new(TestCheckResponse), response)

	httpResponse := &HttpResponse{}
	responses := response.GetResponses()
	assert.NotNil(s.T(), responses)
	assert.Len(s.T(), responses, 1)
	assert.Equal(s.T(), "HttpResponse", responses[0].Response.TypeUrl)

	err = proto.Unmarshal(responses[0].Response.Value, httpResponse)
	assert.Nil(s.T(), err)
}

func (s *CheckerTestSuite) TestCheckResolverFailure() {
	target := &Target{
		Id:   "unknown",
		Type: "sg",
		Name: "unknown",
	}
	request, err := s.buildTestCheckRequest(s.Common.HTTPCheck(), target)
	assert.NoError(s.T(), err)

	response, err := s.CheckerClient.Client.TestCheck(s.Context, request)
	assert.IsType(s.T(), new(TestCheckResponse), response)
	assert.NotNil(s.T(), response.Error)
}

func (s *CheckerTestSuite) TestCheckResolverEmpty() {
	target := &Target{
		Id:   "empty",
		Type: "sg",
		Name: "unknown",
	}
	request, err := s.buildTestCheckRequest(s.Common.HTTPCheck(), target)
	assert.NoError(s.T(), err)

	response, err := s.CheckerClient.Client.TestCheck(s.Context, request)
	assert.NoError(s.T(), err)
	assert.IsType(s.T(), new(TestCheckResponse), response)
	assert.NotNil(s.T(), response.Error)
}

func (s *CheckerTestSuite) TestCheckTimeout() {
	target := &Target{
		Name: "sg",
		Type: "sg",
		Id:   "sg",
	}
	request, err := s.buildTestCheckRequest(s.Common.HTTPCheck(), target)
	assert.NoError(s.T(), err)

	ctx, _ := context.WithDeadline(s.Context, time.Now())

	// We bypass the client here, because it will intercept the context and return
	// an error, but we want to simulate what happens if the deadline is met
	// _after_ we get to the Checker.
	response, err := s.Checker.TestCheck(ctx, request)
	assert.NoError(s.T(), err)
	assert.IsType(s.T(), new(TestCheckResponse), response)
}

func (s *CheckerTestSuite) TestCheckAdheresToMaxHosts() {
	target := &Target{
		Type: "sg",
		Id:   "sg3",
		Name: "sg3",
	}
	request, err := s.buildTestCheckRequest(s.Common.HTTPCheck(), target)
	request.MaxHosts = 1
	assert.NoError(s.T(), err)

	response, err := s.CheckerClient.Client.TestCheck(s.Context, request)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.Len(s.T(), response.GetResponses(), 1)
}

func (s *CheckerTestSuite) TestCheckSupportsInstances() {
	target := &Target{
		Type: "instance",
		Id:   "instance",
		Name: "instance",
	}
	request, err := s.buildTestCheckRequest(s.Common.HTTPCheck(), target)
	request.MaxHosts = 3
	assert.NoError(s.T(), err)

	response, err := s.CheckerClient.Client.TestCheck(s.Context, request)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.Len(s.T(), response.GetResponses(), 1)
}

func (s *CheckerTestSuite) TestUpdateCheck() {
	_, err := s.Checker.UpdateCheck(nil, nil)
	assert.Error(s.T(), err)
}

func TestCheckerTestSuite(t *testing.T) {
	setupTestEnv()
	suite.Run(t, new(CheckerTestSuite))
}
