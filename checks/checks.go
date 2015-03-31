package checks

import (
	"errors"
	"golang.org/x/net/context"
	"time"
)

var errTimeout = errors.New("timedout")
var errClosing = errors.New("closing")

type ResultChan chan CheckResults

type Check interface {
	Context() context.Context
	RunCheck(host string, results ResultChan)
	//    Run(context.Context) context.CancelFunc
}

type CheckResults struct {
	Host     string
	Check    Check
	Err      error
	Response []byte
	Latency  time.Duration
	Context  context.Context
}

type BaseCheck struct {
}
