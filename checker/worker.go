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
}

type Worker interface {
	Work(*Task) *Task
}

type NewWorkerFunc func(chan Worker) Worker

func RegisterWorker(workerType string, newFun NewWorkerFunc) {
	Recruiters[workerType] = newFun
}
