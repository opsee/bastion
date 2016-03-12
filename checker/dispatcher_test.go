package checker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

type dispatcherTestWorkerRequest struct{}
type dispatcherTestWorker struct {
	WorkerQueue chan Worker
}

func (w *dispatcherTestWorkerRequest) Do() *Response {
	return &Response{}
}
func newDispatcherTestWorker(c chan Worker) Worker {
	return &dispatcherTestWorker{
		WorkerQueue: c,
	}
}
func (w *dispatcherTestWorker) Work(t *Task) *Task {
	t.Response = t.Request.Do()
	return t
}

type DispatcherTestSuite struct {
	suite.Suite
	Common      TestCommonStubs
	Dispatcher  *Dispatcher
	Context     context.Context
	MultiTaskTG TaskGroup
	EmptyTG     TaskGroup
}

func (s *DispatcherTestSuite) SetupTest() {
	s.Dispatcher = NewDispatcher()
	s.Context = context.Background()
	RegisterWorker("dispatcherTestWorkerRequest", newDispatcherTestWorker)
	s.MultiTaskTG = TaskGroup{
		&Task{Type: "dispatcherTestWorkerRequest", Request: new(dispatcherTestWorkerRequest)},
		&Task{Type: "dispatcherTestWorkerRequest", Request: new(dispatcherTestWorkerRequest)},
		&Task{Type: "dispatcherTestWorkerRequest", Request: new(dispatcherTestWorkerRequest)},
	}
	s.EmptyTG = TaskGroup{}
}

func (s *DispatcherTestSuite) TestDispatchClosesFinishedChannel() {
	tg := s.EmptyTG
	finished := s.Dispatcher.Dispatch(s.Context, tg)
	select {
	case _, ok := <-finished:
		assert.False(s.T(), ok)
	case <-time.After(time.Duration(1) * time.Second):
		assert.Fail(s.T(), "Dispatcher.Dispatch did not close channel.")
	}
}

func (s *DispatcherTestSuite) TestDispatchEmptyTaskGroup() {
	tg := s.EmptyTG
	finished := s.Dispatcher.Dispatch(s.Context, tg)
	done := TaskGroup{}
	for ft := range finished {
		done = append(done, ft)
	}
	assert.Len(s.T(), tg, 0)
}

func (s *DispatcherTestSuite) TestDispatchTaskGroup() {
	tg := s.MultiTaskTG

	finished := s.Dispatcher.Dispatch(s.Context, tg)
	done := TaskGroup{}
	for ft := range finished {
		done = append(done, ft)
	}
	assert.Len(s.T(), done, 3)
	for _, t := range done {
		assert.IsType(s.T(), new(Task), t)
		assert.NotNil(s.T(), t.Response)
	}
}

func (s *DispatcherTestSuite) TestDispatchWithDeadlineExceeded() {
	tg := s.MultiTaskTG
	ctx, _ := context.WithDeadline(s.Context, time.Now())
	finished := s.Dispatcher.Dispatch(ctx, tg)
	done := TaskGroup{}
	for ft := range finished {
		done = append(done, ft)
	}
	assert.Len(s.T(), done, 3)
	for _, t := range done {
		assert.IsType(s.T(), new(Task), t)
		assert.NotNil(s.T(), t.Response.Error)
	}
}

func (s *DispatcherTestSuite) TestDispatchWithCancelledContext() {
	tg := s.MultiTaskTG
	ctx, cancel := context.WithCancel(s.Context)
	cancel()
	finished := s.Dispatcher.Dispatch(ctx, tg)
	done := TaskGroup{}
	for ft := range finished {
		done = append(done, ft)
	}
	assert.Len(s.T(), done, 3)
	for _, t := range done {
		assert.IsType(s.T(), new(Task), t)
		assert.NotNil(s.T(), t.Response.Error)
	}
}

func TestDispatcherTestSuite(t *testing.T) {
	setupTestEnv()
	suite.Run(t, new(DispatcherTestSuite))
}
