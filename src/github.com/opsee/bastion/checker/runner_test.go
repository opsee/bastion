package checker

import (
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/golang/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"github.com/opsee/basic/schema"
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

func (s *RunnerTestSuite) TestRunnerWorksWithoutSlate() {
	check := s.Common.PassingCheckMultiTarget()
	slate_host := os.Getenv("SLATE_HOST")
	os.Setenv("SLATE_HOST", "")
	assert.Equal(s.T(), "", os.Getenv("SLATE_HOST"))
	runner := NewRunner(s.Resolver)
	responses, err := runner.RunCheck(s.Context, check)
	assert.NoError(s.T(), err)
	targets, err := s.Resolver.Resolve(&schema.Target{
		Id: "sg3",
	})
	assert.NoError(s.T(), err)
	assert.Len(s.T(), targets, 3)

	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(schema.CheckResponse), response)
		assert.NotNil(s.T(), response.Response)
	}
	assert.Equal(s.T(), 3, count)
	os.Setenv("SLATE_HOST", slate_host)
}

func (s *RunnerTestSuite) TestRunCheckHasResponsePerTarget() {
	check := s.Common.PassingCheckMultiTarget()
	responses, err := s.Runner.RunCheck(s.Context, check)
	assert.NoError(s.T(), err)
	targets, err := s.Resolver.Resolve(&schema.Target{
		Id: "sg3",
	})
	assert.NoError(s.T(), err)
	assert.Len(s.T(), targets, 3)

	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(schema.CheckResponse), response)
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
		assert.IsType(s.T(), new(schema.CheckResponse), response)
		assert.NotNil(s.T(), response.Response)
	}
	assert.Equal(s.T(), 1, count)
}

func (s *RunnerTestSuite) TestRunCheckCanCheckAnInstanceTarget() {
	ctx := context.WithValue(s.Context, "MaxHosts", 3)
	check := s.Common.PassingCheckInstanceTarget()
	responses, err := s.Runner.RunCheck(ctx, check)
	assert.NoError(s.T(), err)
	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(schema.CheckResponse), response)
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
		assert.IsType(s.T(), new(schema.CheckResponse), response)
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
		assert.IsType(s.T(), new(schema.CheckResponse), response)
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
	Common         TestCommonStubs
	Runner         *NSQRunner
	Context        context.Context
	Resolver       *testResolver
	Consumer       *nsq.Consumer
	Producer       *nsq.Producer
	MsgChan        chan *nsq.Message
	Config         *NSQRunnerConfig
	ResetNsqConfig resetNsqConfig
}

func (s *NSQRunnerTestSuite) SetupSuite() {
	s.Resolver = newTestResolver()
	s.Common = TestCommonStubs{}
	s.Context = context.Background()
	cfg := &NSQRunnerConfig{
		ConsumerQueueName:   "test-runner",
		ProducerQueueName:   "test-results",
		ConsumerChannelName: "test-runner",
		NSQDHost:            os.Getenv("NSQD_HOST"),
		CustomerID:          "test",
		MaxHandlers:         1,
	}
	s.Config = cfg

	s.ResetNsqConfig = resetNsqConfig{
		Topics: []NsqTopic{
			NsqTopic{s.Config.ConsumerQueueName},
			NsqTopic{s.Config.ProducerQueueName},
		},
		Channels: []NsqChannel{
			NsqChannel{s.Config.ConsumerQueueName, s.Config.ConsumerChannelName},
			NsqChannel{s.Config.ProducerQueueName, "test-runner-results"},
		},
	}
}

func (s *NSQRunnerTestSuite) SetupTest() {
	resetNsq(strings.Split(s.Config.NSQDHost, ":")[0], s.ResetNsqConfig)
	s.MsgChan = make(chan *nsq.Message, 1)

	// Connect our consumer to the producer channel for the NSQRunner.
	consumer, err := nsq.NewConsumer(s.Config.ProducerQueueName, "test-runner-results", nsq.NewConfig())
	if err != nil {
		panic(err)
	}
	consumer.AddConcurrentHandlers(nsq.HandlerFunc(func(m *nsq.Message) error {
		log.Debug("Test consumer handling message: %s", m.Body)
		s.MsgChan <- m
		log.Debug("Test consumer sent message on message channel.")
		return nil
	}), s.Config.MaxHandlers)
	err = consumer.ConnectToNSQD(s.Config.NSQDHost)
	if err != nil {
		panic(err)
	}
	s.Consumer = consumer
	producer, err := nsq.NewProducer(s.Config.NSQDHost, nsq.NewConfig())
	if err != nil {
		panic(err)
	}
	s.Producer = producer

	runner, err := NewNSQRunner(NewRunner(s.Resolver), s.Config)
	if err != nil {
		panic(err)
	}
	s.Runner = runner

}

func (s *NSQRunnerTestSuite) TearDownTest() {
	s.Runner.Stop()
	s.Consumer.Stop()
	<-s.Consumer.StopChan
	s.Producer.Stop()
	close(s.MsgChan)
	resetNsq(strings.Split(s.Config.NSQDHost, ":")[0], s.ResetNsqConfig)
}

func (s *NSQRunnerTestSuite) TestHandlerDoesItsThing() {
	check := s.Common.PassingCheck()
	msg, _ := proto.Marshal(check)
	s.Producer.Publish(s.Config.ConsumerQueueName, msg)
	select {
	case m := <-s.MsgChan:
		log.Debug("TestHandlerDoesItsThing: Received message.")
		result := &schema.CheckResult{}
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
