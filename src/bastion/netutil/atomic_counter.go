package netutil

import "sync/atomic"

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
