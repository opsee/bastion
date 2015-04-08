package netutil

import (
	"github.com/opsee/bastion/Godeps/_workspace/src/golang.org/x/net/context"
	"time"
)

type Context struct {
	context.Context
}

func wrap(context context.Context) Context {
	return Context{context}
}

func TODO() Context {
	return wrap(context.TODO())
}

func Background() Context {
	return wrap(context.Background())
}

func WithCancel(parent Context) (Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent.Context)
	return wrap(ctx), cancel
}

func WithTimeout(parent Context, timeout time.Duration) (Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent.Context, timeout)
	return wrap(ctx), cancel
}

func WithDeadline(parent Context, timeout time.Time) (Context, context.CancelFunc) {
	ctx, cancel := context.WithDeadline(parent.Context, timeout)
	return wrap(ctx), cancel
}

func WithValue(parent Context, key interface{}, value interface{}) Context {
	var data map[interface{}]interface{}
	if parent.Value(0); data != nil {
		return wrap(context.WithValue(parent, 0, parent.Value(0).(map[interface{}]interface{})))
	} else {
		return wrap(context.WithValue(parent, 0, make(map[interface{}]interface{})))
	}
}
