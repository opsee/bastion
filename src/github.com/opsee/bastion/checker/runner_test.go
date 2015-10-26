package checker

import (
	"os"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

type RunnerTestSuite struct {
	suite.Suite
	Common   TestCommonStubs
	Runner   *Runner
	Context  context.Context
	Resolver *testResolver
}

func (s *RunnerTestSuite) SetupTest() {
	s.Resolver = newTestResolver()
	s.Runner = NewRunner(s.Resolver)
	s.Common = TestCommonStubs{}
	s.Context = context.Background()
}

func (s *RunnerTestSuite) TestRunCheckHasResponsePerTarget() {
	check := s.Common.PassingCheckMultiTarget()
	responses, err := s.Runner.RunCheck(s.Context, check)
	assert.NoError(s.T(), err)
	targets, err := s.Resolver.Resolve(&Target{
		Id: "sg3",
	})
	assert.NoError(s.T(), err)
	assert.Len(s.T(), targets, 3)

	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(CheckResponse), response)
		assert.NotNil(s.T(), response.Response)
	}
	assert.Equal(s.T(), 3, count)
}

func (s *RunnerTestSuite) TestRunCheckAdheresToMaxHosts() {
	ctx := context.WithValue(s.Context, "MaxHosts", 1)
	check := s.Common.PassingCheckMultiTarget()
	responses, err := s.Runner.RunCheck(ctx, check)
	assert.NoError(s.T(), err)
	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(CheckResponse), response)
		assert.NotNil(s.T(), response.Response)
	}
	assert.Equal(s.T(), 1, count)
}

func (s *RunnerTestSuite) TestRunCheckClosesChannel() {
	check := s.Common.PassingCheckMultiTarget()
	responses, err := s.Runner.RunCheck(s.Context, check)
	assert.NoError(s.T(), err)
	for {
		select {
		case r, ok := <-responses:
			if !ok {
				return
			}
			assert.NotNil(s.T(), r.Response)
		case <-time.After(time.Duration(5) * time.Second):
			assert.Fail(s.T(), "Timed out waiting for response channel to close.")
		}
	}
}

func (s *RunnerTestSuite) TestRunCheckDeadlineExceeded() {
	ctx, _ := context.WithDeadline(s.Context, time.Unix(0, 0))
	check := s.Common.PassingCheckMultiTarget()
	responses, err := s.Runner.RunCheck(ctx, check)
	assert.NoError(s.T(), err)
	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(CheckResponse), response)
		assert.NotNil(s.T(), response.Error)
	}
	assert.Equal(s.T(), 3, count)
}

func (s *RunnerTestSuite) TestRunCheckCancelledContext() {
	ctx, cancel := context.WithCancel(s.Context)
	cancel()
	check := s.Common.PassingCheckMultiTarget()
	responses, err := s.Runner.RunCheck(ctx, check)
	assert.NoError(s.T(), err)
	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(CheckResponse), response)
		assert.NotNil(s.T(), response.Error)
	}
	assert.Equal(s.T(), 3, count)
}

func (s *RunnerTestSuite) TestRunCheckResolveFailureReturnsError() {
	check := s.Common.BadCheck()
	responses, err := s.Runner.RunCheck(s.Context, check)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), responses)
}

func (s *RunnerTestSuite) TestRunCheckBadCheckReturnsError() {
	check := s.Common.BadCheck()
	responses, err := s.Runner.RunCheck(s.Context, check)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), responses)
}

func TestRunnerTestSuite(t *testing.T) {
	setupTestEnv()
	suite.Run(t, new(RunnerTestSuite))
}

/*******************************************************************************
 * NSQ Runner
 ******************************************************************************/

type NSQRunnerTestSuite struct {
	suite.Suite
	Common   TestCommonStubs
	Runner   *NSQRunner
	Context  context.Context
	Resolver *testResolver
	Consumer *nsq.Consumer
	Producer *nsq.Producer
	MsgChan  chan *nsq.Message
	Config   *NSQRunnerConfig
}

func (s *NSQRunnerTestSuite) SetupSuite() {
	s.Resolver = newTestResolver()
	s.Common = TestCommonStubs{}
	s.Context = context.Background()
	s.MsgChan = make(chan *nsq.Message, 1)
	cfg := &NSQRunnerConfig{
		ConsumerQueueName:   "test-runner",
		ProducerQueueName:   "test-results",
		ConsumerChannelName: "test-runner",
		NSQDHost:            os.Getenv("NSQD_HOST"),
		CustomerID:          "test",
		MaxHandlers:         1,
	}

	// Connect our consumer to the producer channel for the NSQRunner.
	consumer, _ := nsq.NewConsumer(cfg.ProducerQueueName, "test-runner-results", nsq.NewConfig())
	consumer.AddConcurrentHandlers(nsq.HandlerFunc(func(m *nsq.Message) error {
		s.MsgChan <- m
		return nil
	}), cfg.MaxHandlers)
	err := consumer.ConnectToNSQD(cfg.NSQDHost)
	if err != nil {
		panic(err)
	}
	s.Consumer = consumer
	producer, err := nsq.NewProducer(cfg.NSQDHost, nsq.NewConfig())
	if err != nil {
		panic(err)
	}
	s.Producer = producer
	s.Config = cfg
}

func (s *NSQRunnerTestSuite) SetupTest() {
	runner, err := NewNSQRunner(NewRunner(s.Resolver), s.Config)
	if err != nil {
		panic(err)
	}
	s.Runner = runner
}

func (s *NSQRunnerTestSuite) TearDownTest() {
	s.Runner.Stop()
}

func (s *NSQRunnerTestSuite) TearDownSuite() {
	s.Consumer.Stop()
	<-s.Consumer.StopChan
	s.Producer.Stop()
	close(s.MsgChan)
}

func (s *NSQRunnerTestSuite) TestHandlerDoesItsThing() {
	check := s.Common.PassingCheck()
	msg, _ := proto.Marshal(check)
	s.Producer.Publish(s.Config.ConsumerQueueName, msg)
	select {
	case m := <-s.MsgChan:
		result := &CheckResult{}
		err := proto.Unmarshal(m.Body, result)
		assert.NoError(s.T(), err)
		assert.Equal(s.T(), check.Id, result.CheckId)
	case <-time.After(10 * time.Second):
		assert.Fail(s.T(), "Timed out waiting on response from runner.")
	}
}

func TestNSQRunnerTestSuite(t *testing.T) {
	setupTestEnv()
	suite.Run(t, new(NSQRunnerTestSuite))
}
