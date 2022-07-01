// Package server provides runtime support for implementing Cap'n Proto
// interfaces locally.
package server // import "capnproto.org/go/capnp/v3/server"

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"capnproto.org/go/capnp/v3"
	"capnproto.org/go/capnp/v3/exc"
	"capnproto.org/go/capnp/v3/internal/mpsc"
)

// A Method describes a single capability method on a server object.
type Method struct {
	capnp.Method
	Impl func(context.Context, *Call) error
}

// Call holds the state of an ongoing capability method call.
// A Call cannot be used after the server method returns.
type Call struct {
	ctx    context.Context
	cancel context.CancelFunc
	method *Method
	recv   capnp.Recv
	aq     *answerQueue
	srv    *Server

	alloced bool
	results capnp.Struct

	acked bool
}

// Args returns the call's arguments.  Args is not safe to
// reference after a method implementation returns.  Args is safe to
// call and read from multiple goroutines.
func (c *Call) Args() capnp.Struct {
	return c.recv.Args
}

// AllocResults allocates the results struct.  It is an error to call
// AllocResults more than once.
func (c *Call) AllocResults(sz capnp.ObjectSize) (capnp.Struct, error) {
	if c.alloced {
		return capnp.Struct{}, newError("multiple calls to AllocResults")
	}
	var err error
	c.alloced = true
	c.results, err = c.recv.Returner.AllocResults(sz)
	return c.results, err
}

// Ack is a function that is called to acknowledge the delivery of the
// RPC call, allowing other RPC methods to be called on the server.
// After the first call, subsequent calls to Ack do nothing.
//
// Ack need not be the first call in a function nor is it required.
// Since the function's return is also an acknowledgment of delivery,
// short functions can return without calling Ack.  However, since
// the server will not return an Answer until the delivery is
// acknowledged, failure to acknowledge a call before waiting on an
// RPC may cause deadlocks.
func (c *Call) Ack() {
	if c.acked {
		return
	}
	c.acked = true
	go c.srv.handleCalls(c.srv.handleCallsCtx)
}

// Shutdowner is the interface that wraps the Shutdown method.
type Shutdowner interface {
	Shutdown()
}

// A Server is a locally implemented interface.  It implements the
// capnp.ClientHook interface.
type Server struct {
	methods  sortedMethods
	brand    interface{}
	shutdown Shutdowner

	// Cancels handleCallsCtx
	cancelHandleCalls context.CancelFunc

	// Context used by the goroutine running handleCalls(). Note
	// the calls themselves will have different contexts, which
	// are not children of this context, but are supplied by
	// start(). See cancelCurrentCall.
	handleCallsCtx context.Context

	// wg is incremented each time a method is queued, and
	// decremented after it is handled.
	wg sync.WaitGroup

	// Calls are inserted into this queue, to be handled
	// by a goroutine running handleCalls()
	callQueue *mpsc.Queue[*Call]

	// When a call is in progress, this channel will contain the
	// CancelFunc for that call's context. A goroutine may receive
	// on this to fetch the function, and is then responsible for calling
	// it. This happens in Shutdown().
	//
	// The caller must call cancelHandleCalls() *before* calling
	// the received CancelFunc.
	cancelCurrentCall chan context.CancelFunc
}

// Policy is a set of behavioral parameters for a Server.
// They're not specific to a particular server and are generally set at
// an application level.  Library functions are encouraged to accept a
// Policy from a caller instead of creating their own.
type Policy struct {
	// MaxConcurrentCalls is the maximum number of methods allowed to be
	// executing on a single Server simultaneously.  Attempts to make more
	// calls than this limit will result in immediate error answers.
	//
	// If this is zero, then a reasonably small default is used.
	MaxConcurrentCalls int
}

// New returns a client hook that makes calls to a set of methods.
// If shutdown is nil then the server's shutdown is a no-op.  The server
// guarantees message delivery order by blocking each call on the
// return or acknowledgment of the previous call.  See Call.Ack for more
// details.
func New(methods []Method, brand interface{}, shutdown Shutdowner, policy *Policy) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	srv := &Server{
		methods:           make(sortedMethods, len(methods)),
		brand:             brand,
		shutdown:          shutdown,
		callQueue:         mpsc.New[*Call](),
		cancelHandleCalls: cancel,
		handleCallsCtx:    ctx,
		cancelCurrentCall: make(chan context.CancelFunc, 1),
	}
	copy(srv.methods, methods)
	sort.Sort(srv.methods)
	go srv.handleCalls(ctx)
	return srv
}

// Send starts a method call.
func (srv *Server) Send(ctx context.Context, s capnp.Send) (*capnp.Answer, capnp.ReleaseFunc) {
	mm := srv.methods.find(s.Method)
	if mm == nil {
		return capnp.ErrorAnswer(s.Method, capnp.Unimplemented("unimplemented")), func() {}
	}
	args, err := sendArgsToStruct(s)
	if err != nil {
		return capnp.ErrorAnswer(mm.Method, err), func() {}
	}
	ret := new(structReturner)
	return ret.answer(mm.Method, srv.start(ctx, mm, capnp.Recv{
		Method: mm.Method, // pick up names from server method
		Args:   args,
		ReleaseArgs: func() {
			if msg := args.Message(); msg != nil {
				msg.Reset(nil)
				args = capnp.Struct{}
			}
		},
		Returner: ret,
	}))
}

// Recv starts a method call.
func (srv *Server) Recv(ctx context.Context, r capnp.Recv) capnp.PipelineCaller {
	mm := srv.methods.find(r.Method)
	if mm == nil {
		r.Reject(capnp.Unimplemented("unimplemented"))
		return nil
	}
	return srv.start(ctx, mm, r)
}

func (srv *Server) handleCalls(ctx context.Context) {
	for {
		call, err := srv.callQueue.Recv(ctx)
		if err != nil {
			break
		}

		srv.handleCall(ctx, call)
		if call.acked {
			// Another goroutine has taken over; time
			// to retire.
			return
		}
	}
	for {
		// Context has been canceled; drain the rest of the queue,
		// cancelling each call.
		call, ok := srv.callQueue.TryRecv()
		if !ok {
			return
		}
		call.cancel()
		srv.handleCall(ctx, call)
	}
}

func (srv *Server) handleCall(ctx context.Context, c *Call) {
	defer srv.wg.Done()
	defer c.cancel()

	// Store this in the channel, in case Shutdown() gets called
	// while we're servicing the method call.
	srv.cancelCurrentCall <- c.cancel
	defer func() {
		select {
		case <-srv.cancelCurrentCall:
		default:
		}
	}()

	// Handling the contexts is tricky here, since neither one
	// is necessarily a parent of the other. We need to check
	// the context that was passed to us (which manages the
	// handleCalls loop) some time *after* storing c.cancel,
	// above, to avoid a race between this code and Shutdown(),
	// which cancels ctx before attempting to receive c.cancel.
	err := ctx.Err()
	if err == nil {
		err = c.ctx.Err()
	}
	if err == nil {
		err = c.method.Impl(c.ctx, c)
	}

	c.recv.ReleaseArgs()
	if err == nil {
		c.aq.fulfill(c.results)
	} else {
		c.aq.reject(err)
	}
	c.recv.Returner.Return(err)
}

func (srv *Server) start(ctx context.Context, m *Method, r capnp.Recv) capnp.PipelineCaller {
	srv.wg.Add(1)

	ctx, cancel := context.WithCancel(ctx)

	aq := newAnswerQueue(r.Method)
	srv.callQueue.Send(&Call{
		ctx:    ctx,
		cancel: cancel,
		method: m,
		recv:   r,
		aq:     aq,
		srv:    srv,
	})
	return aq
}

// Brand returns a value that will match IsServer.
func (srv *Server) Brand() capnp.Brand {
	return capnp.Brand{Value: serverBrand{srv.brand}}
}

// Shutdown waits for ongoing calls to finish and calls Shutdown on the
// Shutdowner passed into NewServer.  Shutdown must not be called more
// than once.
func (srv *Server) Shutdown() {
	// Cancel the loop in handleCalls(), and then cancel the outstanding
	// call, if any. The order here is critical; if we cancel the
	// outstanding call first, the loop may start another call before
	// we cancel it.
	srv.cancelHandleCalls()
	select {
	case cancel := <-srv.cancelCurrentCall:
		cancel()
	default:
	}

	srv.wg.Wait()
	if srv.shutdown != nil {
		srv.shutdown.Shutdown()
	}
}

// IsServer reports whether a brand returned by capnp.Client.Brand
// originated from Server.Brand, and returns the brand argument passed
// to New.
func IsServer(brand capnp.Brand) (_ interface{}, ok bool) {
	sb, ok := brand.Value.(serverBrand)
	return sb.x, ok
}

type serverBrand struct {
	x interface{}
}

func sendArgsToStruct(s capnp.Send) (capnp.Struct, error) {
	if s.PlaceArgs == nil {
		return capnp.Struct{}, nil
	}
	st, err := newBlankStruct(s.ArgsSize)
	if err != nil {
		return capnp.Struct{}, err
	}
	if err := s.PlaceArgs(st); err != nil {
		st.Message().Reset(nil)
		// Using fmt.Errorf to ensure sendArgsToStruct returns a generic error.
		return capnp.Struct{}, fmt.Errorf("place args: %v", err)
	}
	return st, nil
}

func newBlankStruct(sz capnp.ObjectSize) (capnp.Struct, error) {
	_, seg, err := capnp.NewMessage(capnp.MultiSegment(nil))
	if err != nil {
		return capnp.Struct{}, err
	}
	st, err := capnp.NewRootStruct(seg, sz)
	if err != nil {
		return capnp.Struct{}, err
	}
	return st, nil
}

type sortedMethods []Method

// find returns the method with the given ID or nil.
func (sm sortedMethods) find(id capnp.Method) *Method {
	i := sort.Search(len(sm), func(i int) bool {
		m := &sm[i]
		if m.InterfaceID != id.InterfaceID {
			return m.InterfaceID >= id.InterfaceID
		}
		return m.MethodID >= id.MethodID
	})
	if i == len(sm) {
		return nil
	}
	m := &sm[i]
	if m.InterfaceID != id.InterfaceID || m.MethodID != id.MethodID {
		return nil
	}
	return m
}

func (sm sortedMethods) Len() int {
	return len(sm)
}

func (sm sortedMethods) Less(i, j int) bool {
	if id1, id2 := sm[i].InterfaceID, sm[j].InterfaceID; id1 != id2 {
		return id1 < id2
	}
	return sm[i].MethodID < sm[j].MethodID
}

func (sm sortedMethods) Swap(i, j int) {
	sm[i], sm[j] = sm[j], sm[i]
}

type resultsAllocer interface {
	AllocResults(capnp.ObjectSize) (capnp.Struct, error)
}

func newError(msg string) error {
	return exc.New(exc.Failed, "capnp server", msg)
}

func errorf(format string, args ...interface{}) error {
	return newError(fmt.Sprintf(format, args...))
}
