package checks

import (
		"time"
		"fmt"
		"net"
)

// net.Dial will happily return a net.Conn even if the other side hasn't accepted.
// This sucks and is not the behavior we want, so we have to use syscalls and the
// runtime polling intrinsics to make this right.

type TcpCheck struct {
	port		int
	timeout 	time.Duration
}

type runningTcpCheck struct {
	host 		string
	check 		*TcpCheck
	port 		int
	resultsChan chan CheckResults

}

func (t *TcpCheck) RunCheck(host string, results chan CheckResults) {
	runner := &runningTcpCheck{host, t, t.port, results}
	go runTCP(runner)
}

func runTCP(t *runningTcpCheck) {
	results := CheckResults{Host : t.host, Check : t.check}
	startTime := time.Now()
	tcp := t.check
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", t.host, tcp.port), tcp.timeout)
	if err != nil {
		errAndSend(&results, err, startTime, t)
		return
	} else {
		results.Latency = time.Since(startTime)
		t.resultsChan <- results
	}
	defer conn.Close()
}

func errAndSend(results *CheckResults, err error, start time.Time, t *runningTcpCheck) {
	results.Err = err
	results.Latency = time.Since(start)
	t.resultsChan <- *results
}