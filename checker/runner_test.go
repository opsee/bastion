package checker

import (
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/golang/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"github.com/opsee/basic/schema"
	"github.com/opsee/bastion/config"
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
	s.Runner = NewRunner(&schema.HttpCheck{})
	s.Common = TestCommonStubs{}
	s.Context = context.Background()
}

func (s *RunnerTestSuite) TestRunnerWorksWithoutSlate() {
	check := s.Common.PassingCheckMultiTarget()
	// Not updating this to use the global config
	slate_host := os.Getenv("SLATE_HOST")
	os.Setenv("SLATE_HOST", "")
	assert.Equal(s.T(), "", os.Getenv("SLATE_HOST"))
	runner := NewRunner(&schema.HttpCheck{})
	targets, err := s.Resolver.Resolve(s.Context, &schema.Target{
		Id: "sg3",
	})

	responses, err := runner.RunCheck(s.Context, check, targets)
	assert.NoError(s.T(), err)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), targets, 3)

	for _, response := range responses {
		assert.IsType(s.T(), new(schema.CheckResponse), response)
		assert.NotNil(s.T(), response.Reply)
	}
	assert.Equal(s.T(), 3, len(responses))
	os.Setenv("SLATE_HOST", slate_host)
}

func (s *RunnerTestSuite) TestRunCheckHasResponsePerTarget() {
	check := s.Common.PassingCheckMultiTarget()
	targets, err := s.Resolver.Resolve(s.Context, &schema.Target{
		Id: "sg3",
	})
	responses, err := s.Runner.RunCheck(s.Context, check, targets)
	assert.NoError(s.T(), err)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), targets, 3)

	for _, response := range responses {
		assert.IsType(s.T(), new(schema.CheckResponse), response)
		assert.NotNil(s.T(), response.Reply)
	}
	assert.Equal(s.T(), 3, len(responses))
}

func (s *RunnerTestSuite) TestRunCheckAdheresToMaxHosts() {
	ctx := context.WithValue(s.Context, "MaxHosts", 1)
	check := s.Common.PassingCheckMultiTarget()
	targets, err := s.Resolver.Resolve(s.Context, &schema.Target{
		Id: "sg3",
	})
	if err != nil {
		log.Fatal("Failed to get test targets")
	}

	responses, err := s.Runner.RunCheck(ctx, check, targets)
	assert.NoError(s.T(), err)
	for _, response := range responses {
		assert.IsType(s.T(), new(schema.CheckResponse), response)
		assert.NotNil(s.T(), response.Reply)
	}
	assert.Equal(s.T(), 1, len(responses))
}

func (s *RunnerTestSuite) TestRunCheckCanCheckAnInstanceTarget() {
	ctx := context.WithValue(s.Context, "MaxHosts", 3)
	targets, err := s.Resolver.Resolve(s.Context, &schema.Target{
		Id: "sg",
	})
	if err != nil {
		log.Fatal("Failed to get test targets")
	}

	check := s.Common.PassingCheckInstanceTarget()
	responses, err := s.Runner.RunCheck(ctx, check, targets)
	assert.NoError(s.T(), err)
	for _, response := range responses {
		assert.IsType(s.T(), new(schema.CheckResponse), response)
		assert.NotNil(s.T(), response.Reply)
	}
	assert.Equal(s.T(), 1, len(responses))
}

func (s *RunnerTestSuite) TestRunCheckDeadlineExceeded() {
	ctx, _ := context.WithDeadline(s.Context, time.Unix(0, 0))
	check := s.Common.PassingCheckMultiTarget()
	targets, err := s.Resolver.Resolve(s.Context, &schema.Target{
		Id: "sg3",
	})
	if err != nil {
		log.Fatal("Failed to get test targets")
	}

	responses, err := s.Runner.RunCheck(ctx, check, targets)
	assert.NoError(s.T(), err)
	for _, response := range responses {
		assert.IsType(s.T(), new(schema.CheckResponse), response)
		assert.NotNil(s.T(), response.Error)
	}
	assert.Equal(s.T(), 3, len(responses))
}

func (s *RunnerTestSuite) TestRunCheckCancelledContext() {
	ctx, cancel := context.WithCancel(s.Context)
	cancel()
	check := s.Common.PassingCheckMultiTarget()
	targets, err := s.Resolver.Resolve(s.Context, &schema.Target{
		Id: "sg3",
	})
	if err != nil {
		log.Fatal("Failed to get test targets")
	}

	responses, err := s.Runner.RunCheck(ctx, check, targets)
	assert.NoError(s.T(), err)
	for _, response := range responses {
		assert.IsType(s.T(), new(schema.CheckResponse), response)
		assert.NotNil(s.T(), response.Error)
	}
	assert.Equal(s.T(), 3, len(responses))
}

func (s *RunnerTestSuite) TestRunCheckBadCheckReturnsError() {
	check := s.Common.BadCheck()
	targets, err := s.Resolver.Resolve(s.Context, &schema.Target{
		Id: "sg",
	})
	if err != nil {
		log.Fatal("Failed to get test targets")
	}

	responses, err := s.Runner.RunCheck(s.Context, check, targets)
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
		ProducerNsqdHost:    config.GetConfig().NsqdHost,
		ConsumerNsqdHost:    config.GetConfig().NsqdHost,
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
	resetNsq(strings.Split(s.Config.ConsumerNsqdHost, ":")[0], s.ResetNsqConfig)
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
	err = consumer.ConnectToNSQD(s.Config.ConsumerNsqdHost)
	if err != nil {
		panic(err)
	}
	s.Consumer = consumer
	producer, err := nsq.NewProducer(s.Config.ProducerNsqdHost, nsq.NewConfig())
	if err != nil {
		panic(err)
	}
	s.Producer = producer

	runner, err := NewNSQRunner(NewRunner(&schema.HttpCheck{}), s.Config)
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
	resetNsq(strings.Split(s.Config.ConsumerNsqdHost, ":")[0], s.ResetNsqConfig)
}

func (s *NSQRunnerTestSuite) TestHandlerDoesItsThing() {
	check := s.Common.PassingCheck()
	checkWithTargets, _ := NewCheckTargets(s.Resolver, check)
	msg, _ := proto.Marshal(checkWithTargets)
	s.Producer.Publish(s.Config.ConsumerQueueName, msg)
	timer := time.NewTimer(10 * time.Second)
	select {
	case m := <-s.MsgChan:
		log.Debug("TestHandlerDoesItsThing: Received message.")
		result := &schema.CheckResult{}
		err := proto.Unmarshal(m.Body, result)
		assert.NoError(s.T(), err)
		assert.Equal(s.T(), check.Id, result.CheckId)
	case <-timer.C:
		assert.Fail(s.T(), "Timed out waiting on response from runner.")
	}
	timer.Stop()
}

func (s *NSQRunnerTestSuite) TestResultsHaveCorrectCustomerId() {
	check1 := s.Common.PassingCheck()
	cwt1, _ := NewCheckTargets(s.Resolver, check1)
	msg1, _ := proto.Marshal(cwt1)
	s.Producer.Publish(s.Config.ConsumerQueueName, msg1)

	check2 := s.Common.PassingCheck()
	check2.CustomerId = "check2-customer-id"
	cwt2, _ := NewCheckTargets(s.Resolver, check2)
	msg2, _ := proto.Marshal(cwt2)
	s.Producer.Publish(s.Config.ConsumerQueueName, msg2)

	timer := time.NewTimer(10 * time.Second)

	customerIds := map[string]int{}

	for i := 0; i < 2; i++ {
		select {
		case m := <-s.MsgChan:
			log.Debug("TestHandlerDoesItsThing: Received message.")
			result := &schema.CheckResult{}
			err := proto.Unmarshal(m.Body, result)
			assert.NoError(s.T(), err)
			customerIds[result.CustomerId] += 1
		case <-timer.C:
			assert.Fail(s.T(), "Timed out waiting on response from runner.")
		}
	}

	assert.Equal(s.T(), 1, customerIds["check2-customer-id"])
	assert.Equal(s.T(), 1, customerIds["stub-customer-id"])

	timer.Stop()
}

func TestNSQRunnerTestSuite(t *testing.T) {
	setupTestEnv()
	suite.Run(t, new(NSQRunnerTestSuite))
}
