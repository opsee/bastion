package netutil

import (
	"github.com/op/go-logging"
	"sync/atomic"
)

var (
	log       = logging.MustGetLogger("json-tcp")
	logFormat = logging.MustStringFormatter("%{time:2006-01-02T15:04:05.999999999Z07:00} %{level} [%{module}] %{message}")
)

func init() {
	logging.SetLevel(logging.INFO, "json-tcp")
	logging.SetFormatter(logFormat)
}

type AtomicCounter struct {
	val int64
}

func (counter *AtomicCounter) Load() int64 {
	return atomic.LoadInt64(&counter.val)
}

func (counter *AtomicCounter) Store(val int64) {
	atomic.StoreInt64(&counter.val, val)
}

func (counter *AtomicCounter) Increment() int64 {
	return atomic.AddInt64(&counter.val, 1)
}

func (counter *AtomicCounter) Decrement() int64 {
	return atomic.AddInt64(&counter.val, -1)
}
