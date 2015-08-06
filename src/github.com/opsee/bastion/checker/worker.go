package checker

var (
	Recruiters = make(map[string]NewWorkerFunc)
)

type Request interface {
	Do() (Response, error)
}

type Response interface{}

type ErrorResponse struct {
	Error error `json:"error"`
}

type Task struct {
	Type     string
	Request  Request
	Response chan Response
}

type Worker interface {
	Work()
}

type NewWorkerFunc func(workQueue chan *Task) Worker
