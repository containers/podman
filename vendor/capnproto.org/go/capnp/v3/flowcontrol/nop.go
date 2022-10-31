package flowcontrol

import (
	"context"
)

var (
	// A flow limiter which does not actually limit anything; messages will be
	// sent as fast as possible.
	NopLimiter FlowLimiter = nopLimiter{}
)

type nopLimiter struct{}

func (nopLimiter) StartMessage(context.Context, uint64) (func(), error) {
	return func() {}, nil
}

func (nopLimiter) Release() {}
