package checks

import (
    "errors"
    "time"
)

var errTimeout = errors.New("timedout")
var errClosing = errors.New("closing")

type ResultChan chan CheckResults

type Check interface {
    RunCheck(host string, results ResultChan)
    //    Run(context.Context) context.CancelFunc
}

type CheckResults struct {
    Host     string
    Check    Check
    Err      error
    Response []byte
    Latency  time.Duration
}

type BaseCheck struct {
}
