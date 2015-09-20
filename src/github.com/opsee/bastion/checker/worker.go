package checker

var (
	Recruiters = make(map[string]NewWorkerFunc)
)

type Request interface {
	Do() *Response
}

type Response struct {
	Response interface{}
	Error    error
}

type Task struct {
	Type     string
	Target   *Target
	Request  Request
	Response *Response
	Finished chan *Task
}

type Worker interface {
	Work()
}

type NewWorkerFunc func(workQueue chan *Task) Worker
