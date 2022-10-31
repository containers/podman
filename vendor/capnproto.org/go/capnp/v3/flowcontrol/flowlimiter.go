// Package flowcontrol provides support code for per-object flow control.
//
// This is most important for dealing with streaming interfaces; see
// https://capnproto.org/news/2020-04-23-capnproto-0.8.html#multi-stream-flow-control
// for a description of the general problem.
//
// The Go implementation's approach differs from that of the C++ implementation in that
// we don't treat the `stream` annotation specially; we instead do flow control on all
// objects. Calls to methods will transparently block for the appropriate amount of
// time, so it is safe to simply call rpc methods in a loop.
//
// To change the default flow control policy on a Client, call Client.SetFlowLimiter
// with the desired FlowLimiter.
package flowcontrol

import (
	"context"
)

// A `FlowLimiter` is used to manage flow control for a stream of messages.
type FlowLimiter interface {
	// StartMessage informs the flow limiter that the caller wants to
	// send a message of the specified size. It blocks until an appropriate
	// time to do so, or until the context is canceled. If the returned
	// error is nil, the caller should then proceed in sending the message
	// immediately, and it should arrange to call gotResponse() as soon as
	// a response is received.
	//
	// StartMessage must be safe to call from multiple goroutines.
	StartMessage(ctx context.Context, size uint64) (gotResponse func(), err error)

	// Release releases any resources used by the FlowLimiter.
	Release()
}
