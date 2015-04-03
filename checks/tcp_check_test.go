package checks

import (
	"fmt"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/stretchr/testify/assert"
	nettest "github.com/opsee/bastion/testing/net"
	"net"
	"testing"
	"time"
)

func TestTCPNotListening(t *testing.T) {
	check := TcpCheck{4320, time.Second, []byte{}, []byte{}}
	resultsChan := make(chan CheckResults)
	check.RunCheck("127.0.0.1", resultsChan)
	results := <-resultsChan
	assert.Equal(t, "127.0.0.1", results.Host)
	assert.Equal(t, check, *results.Check.(*TcpCheck))
	err := results.Err.(*net.OpError)
	assert.Equal(t, "dial", err.Op)
	assert.True(t, results.Latency < time.Second)
}

func TestTCPSuccess(t *testing.T) {
	check := TcpCheck{4320, time.Second, []byte("hello"), []byte("fuckoff")}
	resultsChan := make(chan CheckResults)
	testServer := nettest.NetServer(4320, t)
	check.RunCheck("127.0.0.1", resultsChan)
	serverConn := <-testServer.Startup
	fmt.Println("startup")
	buffer := <-testServer.Recv
	assert.Equal(t, []byte("hello"), buffer)
	serverConn.Write([]byte("fuckoff"))
	results := <-resultsChan
	fmt.Println("results")
	assert.Nil(t, results.Err)
	assert.Equal(t, check, *results.Check.(*TcpCheck))
	assert.Equal(t, "127.0.0.1", results.Host)
	assert.Equal(t, []byte("fuckoff"), results.Response)
	assert.True(t, results.Latency < time.Second)
	fmt.Println("closing")
	testServer.Close()
}

func TestTCPNonResponsive(t *testing.T) {
	check := TcpCheck{4320, time.Second, []byte("hello"), []byte("fuckoff")}
	resultsChan := make(chan CheckResults)
	testServer := nettest.NonResponsiveServer(4320, t)
	check.RunCheck("127.0.0.1", resultsChan)
	results := <-resultsChan
	fmt.Println("results", results)
	assert.NotNil(t, results.Err)
	assert.Equal(t, check, *results.Check.(*TcpCheck))
	assert.Equal(t, "127.0.0.1", results.Host)
	assert.InDelta(t, int64(time.Second), int64(results.Latency), float64(100*time.Millisecond))
	testServer.Close()
}
