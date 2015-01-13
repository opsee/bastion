package checks

import (
		"time"
		"errors"
)

var errTimeout = errors.New("timedout")
var errClosing = errors.New("closing")

type Check interface {
	RunCheck(host string, results chan CheckResults)
}

type CheckResults struct {
	Host 		string
	Check 		Check
	Err			error
	Response	[]byte
	Latency		time.Duration	
}