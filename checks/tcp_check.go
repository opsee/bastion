package checks

import (
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"net"
	"time"
)

// net.Dial will happily return a net.Conn even if the other side hasn't accepted.
// This sucks and is not the behavior we want, so we have to use syscalls and the
// runtime polling intrinsics to make this right.

type TcpCheck struct {
	port     int
	timeout  time.Duration
	hello    []byte
	expected []byte
}

func (t *TcpCheck) RunCheck(host string, results ResultChan) {
	go runTCP(host, t, results)
}

func (t *TcpCheck) Context() context.Context {
	return context.Background()
}

func runTCP(host string, check *TcpCheck, resultsChan chan CheckResults) {
	results := CheckResults{Host: host, Check: check}
	startTime := time.Now()
	deadline := startTime.Add(check.timeout)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, check.port), check.timeout)
	if err != nil {
		errAndSend(&results, err, startTime, resultsChan)
		return
	}
	defer conn.Close()
	conn.SetDeadline(deadline)
	n, err := conn.Write(check.hello)
	fmt.Println("wrote hello", n, err, check.hello)
	if err != nil {
		errAndSend(&results, err, startTime, resultsChan)
		return
	}
	if n != len(check.hello) {
		errAndSend(&results, errors.New("Could not write the entire hello"), startTime, resultsChan)
		return
	}
	buffer := make([]byte, len(check.expected))
	n, err = conn.Read(buffer)
	fmt.Println("read response", n, err, check.expected)
	if err != nil {
		errAndSend(&results, err, startTime, resultsChan)
		return
	}
	results.Response = buffer
	if !compare(check.expected, buffer) {
		errAndSend(&results, errors.New("Actual response did not match expected"), startTime, resultsChan)
		return
	}
	resultsChan <- results
}

func compare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i, n := range a {
		if n != b[i] {
			return false
		}
	}
	return true
}

func errAndSend(results *CheckResults, err error, start time.Time, resultsChan chan CheckResults) {
	results.Err = err
	results.Latency = time.Since(start)
	resultsChan <- *results
}
