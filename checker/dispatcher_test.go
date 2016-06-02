package checker

import (
	"testing"
	"time"

	"github.com/opsee/basic/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

type dispatcherTestWorkerRequest struct {
	ID int
}

type dispatcherTestWorker struct {
	WorkerQueue chan Worker
}

func (w *dispatcherTestWorkerRequest) Do(ctx context.Context) <-chan *Response {
	r := make(chan *Response, 1)
	defer close(r)
	r <- &Response{
		// This is real jank, but it is easy.
		Response: &schema.CheckResponse_HttpResponse{&schema.HttpResponse{Code: int32(w.ID)}},
	}
	return r
}
func newDispatcherTestWorker(c chan Worker) Worker {
	return &dispatcherTestWorker{
		WorkerQueue: c,
	}
}
func (w *dispatcherTestWorker) Work(ctx context.Context, t *Task) *Task {
	if ctx.Err() != nil {
		t.Response = &Response{
			Error: ctx.Err(),
		}
		return t
	}
	t.Response = <-t.Request.Do(ctx)
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
	Recruiters.RegisterWorker("dispatcherTestWorkerRequest", newDispatcherTestWorker)
	s.MultiTaskTG = TaskGroup{
		&Task{Type: "dispatcherTestWorkerRequest", Request: &dispatcherTestWorkerRequest{1}},
		&Task{Type: "dispatcherTestWorkerRequest", Request: &dispatcherTestWorkerRequest{2}},
		&Task{Type: "dispatcherTestWorkerRequest", Request: &dispatcherTestWorkerRequest{3}},
	}
	s.EmptyTG = TaskGroup{}
}

func (s *DispatcherTestSuite) TestDispatcherClosesFinishedChannel() {
	tg := s.EmptyTG
	finished := s.Dispatcher.Dispatch(s.Context, tg)
	timer := time.NewTimer(1 * time.Second)
	select {
	case _, ok := <-finished:
		assert.False(s.T(), ok)
	case <-timer.C:
		assert.Fail(s.T(), "Dispatcher.Dispatch did not close channel.")
	}
	timer.Stop()
}

func (s *DispatcherTestSuite) TestDispatcherEmptyTaskGroup() {
	tg := s.EmptyTG
	finished := s.Dispatcher.Dispatch(s.Context, tg)
	done := TaskGroup{}
	for ft := range finished {
		done = append(done, ft)
	}
	assert.Len(s.T(), tg, 0)
}

func (s *DispatcherTestSuite) TestDispatcherTaskGroup() {
	tg := s.MultiTaskTG

	finished := s.Dispatcher.Dispatch(s.Context, tg)
	done := TaskGroup{}
	for ft := range finished {
		done = append(done, ft)
	}
	assert.Len(s.T(), done, 3)
	for _, t := range done {
		req := t.Request.(*dispatcherTestWorkerRequest)
		id := req.ID
		resp, ok := t.Response.Response.(*schema.CheckResponse_HttpResponse)
		assert.True(s.T(), ok)

		assert.EqualValues(s.T(), id, resp.HttpResponse.Code)
		assert.IsType(s.T(), new(Task), t)
		assert.NotNil(s.T(), t.Response)
	}
}

//**********************************************************************************
//
// These context tests are to ensure that things behave deterministically despite the
// non-determinism inherent in the concurrency patterns involved.
//

func (s *DispatcherTestSuite) TestDispatcherWithDeadlineExceeded() {
	tg := s.MultiTaskTG
	ctx, _ := context.WithDeadline(s.Context, time.Now().Add(-1*30*time.Second))
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

func (s *DispatcherTestSuite) TestDispatcherWithCancelledContext() {
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

//
//
//**********************************************************************************

func TestDispatcherTestSuite(t *testing.T) {
	setupTestEnv()
	suite.Run(t, new(DispatcherTestSuite))
}
