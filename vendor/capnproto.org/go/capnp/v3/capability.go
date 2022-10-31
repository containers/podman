package capnp

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"sync"

	"capnproto.org/go/capnp/v3/flowcontrol"
	"capnproto.org/go/capnp/v3/internal/syncutil"
)

func init() {
	close(closedSignal)
}

// An Interface is a reference to a client in a message's capability table.
type Interface struct {
	seg *Segment
	cap CapabilityID
}

// i.EncodeAsPtr is equivalent to i.ToPtr(); for implementing TypeParam.
// The segment argument is ignored.
func (i Interface) EncodeAsPtr(*Segment) Ptr { return i.ToPtr() }

// DecodeFromPtr(p) is equivalent to p.Interface(); for implementing TypeParam.
func (Interface) DecodeFromPtr(p Ptr) Interface { return p.Interface() }

var _ TypeParam[Interface] = Interface{}

// NewInterface creates a new interface pointer.
//
// No allocation is performed in the given segment: it is used purely
// to associate the interface pointer with a message.
func NewInterface(s *Segment, cap CapabilityID) Interface {
	return Interface{s, cap}
}

// ToPtr converts the interface to a generic pointer.
func (i Interface) ToPtr() Ptr {
	return Ptr{
		seg:      i.seg,
		lenOrCap: uint32(i.cap),
		flags:    interfacePtrFlag,
	}
}

// Message returns the message whose capability table the interface
// references or nil if the pointer is invalid.
func (i Interface) Message() *Message {
	if i.seg == nil {
		return nil
	}
	return i.seg.msg
}

// IsValid returns whether the interface is valid.
func (i Interface) IsValid() bool {
	return i.seg != nil
}

// Capability returns the capability ID of the interface.
func (i Interface) Capability() CapabilityID {
	return i.cap
}

// value returns a raw interface pointer with the capability ID.
func (i Interface) value(paddr address) rawPointer {
	if i.seg == nil {
		return 0
	}
	return rawInterfacePointer(i.cap)
}

// Client returns the client stored in the message's capability table
// or nil if the pointer is invalid.
func (i Interface) Client() Client {
	msg := i.Message()
	if msg == nil {
		return Client{}
	}
	tab := msg.CapTable
	if int64(i.cap) >= int64(len(tab)) {
		return Client{}
	}
	return tab[i.cap]
}

// A CapabilityID is an index into a message's capability table.
type CapabilityID uint32

// String returns the ID in the format "capability X".
func (id CapabilityID) String() string {
	return fmt.Sprintf("capability %d", id)
}

// GoString returns the ID as a Go expression.
func (id CapabilityID) GoString() string {
	return fmt.Sprintf("capnp.CapabilityID(%d)", id)
}

// A Client is a reference to a Cap'n Proto capability.
// The zero value is a null capability reference.
// It is safe to use from multiple goroutines.
type Client ClientKind

// The underlying type of Client. We expose this so that
// we can use ~ClientKind as a constraint in generics to
// capture any capability type.
type ClientKind = struct {
	*client
}

type client struct {
	creatorFunc int
	creatorFile string
	creatorLine int

	mu       sync.Mutex // protects the struct
	limiter  flowcontrol.FlowLimiter
	h        *clientHook // nil if resolved to nil or released
	released bool
}

// clientHook is a reference-counted wrapper for a ClientHook.
// It is assumed that a clientHook's address uniquely identifies a hook,
// since they are only created in NewClient and NewPromisedClient.
type clientHook struct {
	// ClientHook will never be nil and will not change for the lifetime of
	// a clientHook.
	ClientHook

	// Place for callers to attach arbitrary metadata to the client.
	metadata Metadata

	// done is closed when refs == 0 and calls == 0.
	done chan struct{}

	// resolved is closed after resolvedHook is set
	resolved chan struct{}

	mu           sync.Mutex
	refs         int         // how many open Clients reference this clientHook
	calls        int         // number of outstanding ClientHook accesses
	resolvedHook *clientHook // valid only if resolved is closed
}

// NewClient creates the first reference to a capability.
// If hook is nil, then NewClient returns nil.
//
// Typically the RPC system will create a client for the application.
// Most applications will not need to use this directly.
func NewClient(hook ClientHook) Client {
	if hook == nil {
		return Client{}
	}
	h := &clientHook{
		ClientHook: hook,
		done:       make(chan struct{}),
		refs:       1,
		resolved:   closedSignal,
		metadata:   *NewMetadata(),
	}
	h.resolvedHook = h
	c := Client{client: &client{h: h}}
	if clientLeakFunc != nil {
		c.creatorFunc = 1
		_, c.creatorFile, c.creatorLine, _ = runtime.Caller(1)
		c.setFinalizer()
	}
	return c
}

// NewPromisedClient creates the first reference to a capability that
// can resolve to a different capability.  The hook will be shut down
// when the promise is resolved or the client has no more references,
// whichever comes first.
//
// Typically the RPC system will create a client for the application.
// Most applications will not need to use this directly.
func NewPromisedClient(hook ClientHook) (Client, *ClientPromise) {
	if hook == nil {
		panic("NewPromisedClient(nil)")
	}
	h := &clientHook{
		ClientHook: hook,
		done:       make(chan struct{}),
		refs:       1,
		resolved:   make(chan struct{}),
		metadata:   *NewMetadata(),
	}
	c := Client{client: &client{h: h}}
	if clientLeakFunc != nil {
		c.creatorFunc = 2
		_, c.creatorFile, c.creatorLine, _ = runtime.Caller(1)
		c.setFinalizer()
	}
	return c, &ClientPromise{h: h}
}

// startCall holds onto a hook to prevent it from shutting down until
// finish is called.  It resolves the client's hook as much as possible
// first.  The caller must not be holding onto c.mu.
func (c Client) startCall() (hook ClientHook, resolved, released bool, finish func()) {
	if c.client == nil {
		return nil, true, false, func() {}
	}
	defer c.mu.Unlock()
	c.mu.Lock()
	if c.h == nil {
		return nil, true, c.released, func() {}
	}
	c.h.mu.Lock()
	c.h = resolveHook(c.h)
	if c.h == nil {
		return nil, true, false, func() {}
	}
	c.h.calls++
	c.h.mu.Unlock()
	savedHook := c.h
	return savedHook.ClientHook, savedHook.isResolved(), false, func() {
		syncutil.With(&savedHook.mu, func() {
			savedHook.calls--
			if savedHook.refs == 0 && savedHook.calls == 0 {
				close(savedHook.done)
			}
		})
	}
}

func (c Client) peek() (hook *clientHook, released bool, resolved bool) {
	if c.client == nil {
		return nil, false, true
	}
	defer c.mu.Unlock()
	c.mu.Lock()
	if c.h == nil {
		return nil, c.released, true
	}
	c.h.mu.Lock()
	c.h = resolveHook(c.h)
	if c.h == nil {
		return nil, false, true
	}
	resolved = c.h.isResolved()
	c.h.mu.Unlock()
	return c.h, false, resolved
}

// resolveHook resolves h as much as possible without blocking.
// The caller must be holding onto h.mu and when resolveHook returns, it
// will be holding onto the mutex of the returned hook if not nil.
func resolveHook(h *clientHook) *clientHook {
	for {
		if !h.isResolved() {
			return h
		}
		r := h.resolvedHook
		if r == h {
			return h
		}
		h.mu.Unlock()
		h = r
		if h == nil {
			return nil
		}
		h.mu.Lock()
	}
}

// Get the current flowcontrol.FlowLimiter used to manage flow control
// for this client.
func (c Client) GetFlowLimiter() flowcontrol.FlowLimiter {
	c.mu.Lock()
	defer c.mu.Unlock()
	ret := c.limiter
	if ret == nil {
		ret = flowcontrol.NopLimiter
	}
	return ret
}

// Update the flowcontrol.FlowLimiter used to manage flow control for
// this client. This affects all future calls, but not calls already
// waiting to send. Passing nil sets the value to flowcontrol.NopLimiter,
// which is also the default.
//
// When .Release() is called on the client, it will call .Release() on
// the FlowLimiter in turn.
func (c Client) SetFlowLimiter(lim flowcontrol.FlowLimiter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.limiter = lim
}

// SendCall allocates space for parameters, calls args.Place to fill out
// the parameters, then starts executing a method, returning an answer
// that will hold the result.  The caller must call the returned release
// function when it no longer needs the answer's data.
//
// This method respects the flow control policy configured with SetFlowLimiter;
// it may block if the sender is sending too fast.
func (c Client) SendCall(ctx context.Context, s Send) (*Answer, ReleaseFunc) {
	h, _, released, finish := c.startCall()
	defer finish()
	if released {
		return ErrorAnswer(s.Method, errorf("call on released client")), func() {}
	}
	if h == nil {
		return ErrorAnswer(s.Method, errorf("call on null client")), func() {}
	}

	limiter := c.GetFlowLimiter()

	// We need to call PlaceArgs before we will know the size of message for
	// flow control purposes, so wrap it in a function that measures after the
	// arguments have been placed:
	placeArgs := s.PlaceArgs
	var size uint64
	s.PlaceArgs = func(args Struct) error {
		var err error
		if placeArgs != nil {
			err = placeArgs(args)
			if err != nil {
				return err
			}
		}

		size, err = args.Segment().Message().TotalSize()
		return err
	}

	ans, rel := h.Send(ctx, s)
	// FIXME: an earlier version of this code called StartMessage() from
	// within PlaceArgs -- but that can result in a deadlock, since it means
	// the client hook is holding a lock while we're waiting on the limiter.
	//
	// As a temporary workaround, we instead do StartMessage *after* the send.
	// This still has a bug, but a much less serious one: we may slightly
	// over-use our limit, but only by the size of a single message. This is
	// mostly a problem in that it contradicts the documentation and is
	// conceptually odd.
	//
	// Longer term, we should fix a more serious design problem: Send() is
	// holding a lock while calling into user code (PlaceArgs), so this
	// deadlock could also arise if the user code blocks. Once that is solved,
	// we can back out this hack.
	gotResponse, err := limiter.StartMessage(ctx, size)
	if err != nil {
		// HACK: An error should only happen if the context was cancelled,
		// in which case the caller will notice it soon probably. The call
		// still went off ok, so we can just return the result we already
		// got, and trying to report the error is awkward because we can't
		// return one... so we don't. Set gotResponse to something that won't
		// break things, and call it a day. See comments above about a
		// longer term solution to this mess.
		gotResponse = func() {}
	}
	p := ans.f.promise
	p.mu.Lock()
	if p.isResolved() {
		// Wow, that was fast.
		p.mu.Unlock()
		gotResponse()
	} else {
		p.signals = append(p.signals, gotResponse)
		p.mu.Unlock()
	}

	return ans, rel
}

// RecvCall starts executing a method with the referenced arguments
// and returns an answer that will hold the result.  The hook will call
// a.Release when it no longer needs to reference the parameters.  The
// caller must call the returned release function when it no longer
// needs the answer's data.
//
// Note that unlike SendCall, this method does *not* respect the flow
// control policy configured with SetFlowLimiter.
func (c Client) RecvCall(ctx context.Context, r Recv) PipelineCaller {
	h, _, released, finish := c.startCall()
	defer finish()
	if released {
		r.Reject(errorf("call on released client"))
		return nil
	}
	if h == nil {
		r.Reject(errorf("call on null client"))
		return nil
	}
	return h.Recv(ctx, r)
}

// IsValid reports whether c is a valid reference to a capability.
// A reference is invalid if it is nil, has resolved to null, or has
// been released.
func (c Client) IsValid() bool {
	h, released, _ := c.peek()
	return !released && h != nil
}

// IsSame reports whether c and c2 refer to a capability created by the
// same call to NewClient.  This can return false negatives if c or c2
// are not fully resolved: use Resolve if this is an issue.  If either
// c or c2 are released, then IsSame panics.
func (c Client) IsSame(c2 Client) bool {
	h1, released, _ := c.peek()
	if released {
		panic("IsSame on released client")
	}
	h2, released, _ := c2.peek()
	if released {
		panic("IsSame on released client")
	}
	return h1 == h2
}

// Resolve blocks until the capability is fully resolved or the Context is Done.
func (c Client) Resolve(ctx context.Context) error {
	for {
		h, released, resolved := c.peek()
		if released {
			return errorf("cannot resolve released client")
		}

		if resolved {
			return nil
		}

		select {
		case <-h.resolved:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// AddRef creates a new Client that refers to the same capability as c.
// If c is nil or has resolved to null, then AddRef returns nil.
func (c Client) AddRef() Client {
	if c.client == nil {
		return Client{}
	}
	defer c.mu.Unlock()
	c.mu.Lock()
	if c.released {
		panic("AddRef on released client")
	}
	if c.h == nil {
		return Client{}
	}
	c.h.mu.Lock()
	c.h = resolveHook(c.h)
	if c.h == nil {
		return Client{}
	}
	c.h.refs++
	c.h.mu.Unlock()
	d := Client{client: &client{h: c.h}}
	if clientLeakFunc != nil {
		d.creatorFunc = 3
		_, d.creatorFile, d.creatorLine, _ = runtime.Caller(1)
		d.setFinalizer()
	}
	return d
}

// WeakRef creates a new WeakClient that refers to the same capability
// as c.  If c is nil or has resolved to null, then WeakRef returns nil.
func (c Client) WeakRef() *WeakClient {
	h, released, _ := c.peek()
	if released {
		panic("WeakRef on released client")
	}
	if h == nil {
		return nil
	}
	return &WeakClient{h: h}
}

// State reads the current state of the client.  It returns the zero
// ClientState if c is nil, has resolved to null, or has been released.
func (c Client) State() ClientState {
	h, resolved, _, finish := c.startCall()
	defer finish()
	if h == nil {
		return ClientState{}
	}
	return ClientState{
		Brand:     h.Brand(),
		IsPromise: !resolved,
		Metadata:  &resolveHook(c.h).metadata,
	}
}

// A Brand is an opaque value used to identify a capability.
type Brand struct {
	Value interface{}
}

// ClientState is a snapshot of a client's identity.
type ClientState struct {
	// Brand is the value returned from the hook's Brand method.
	Brand Brand
	// IsPromise is true if the client has not resolved yet.
	IsPromise bool
	// Arbitrary metadata. Note that, if a Client is a promise,
	// when it resolves its metadata will be replaced with that
	// of its resolution.
	//
	// TODO: this might change before the v3 API is stabilized;
	// we are not sure the above is the correct semantics.
	Metadata *Metadata
}

// String returns a string that identifies this capability for debugging
// purposes.  Its format should not be depended on: in particular, it
// should not be used to compare clients.  Use IsSame to compare clients
// for equality.
func (c Client) String() string {
	if c.client == nil {
		return "<nil>"
	}
	c.mu.Lock()
	if c.released {
		c.mu.Unlock()
		return "<released client>"
	}
	if c.h == nil {
		c.mu.Unlock()
		return "<nil>"
	}
	c.h.mu.Lock()
	c.h = resolveHook(c.h)
	if c.h == nil {
		c.mu.Unlock()
		return "<nil>"
	}
	var s string
	if c.h.isResolved() {
		s = fmt.Sprintf("<client %T@%p>", c.h.ClientHook, c.h)
	} else {
		s = fmt.Sprintf("<unresolved client %T@%p>", c.h.ClientHook, c.h)
	}
	c.h.mu.Unlock()
	c.mu.Unlock()
	return s
}

// Release releases a capability reference.  If this is the last
// reference to the capability, then the underlying resources associated
// with the capability will be released.
//
// Release has no effect if c has already been released, or if c is
// nil or resolved to null.
func (c Client) Release() {
	if c.client == nil {
		return
	}
	c.mu.Lock()
	if c.released || c.h == nil {
		c.mu.Unlock()
		return
	}
	c.released = true
	c.h.mu.Lock()
	c.h = resolveHook(c.h)
	if c.h == nil {
		c.mu.Unlock()
		return
	}
	h := c.h
	c.h = nil
	h.refs--
	if h.refs > 0 {
		h.mu.Unlock()
		c.mu.Unlock()
		return
	}
	if h.calls == 0 {
		close(h.done)
	}
	h.mu.Unlock()
	c.mu.Unlock()
	<-h.done
	h.Shutdown()
	c.GetFlowLimiter().Release()
}

func (c Client) EncodeAsPtr(seg *Segment) Ptr {
	capId := seg.Message().AddCap(c)
	return NewInterface(seg, capId).ToPtr()
}

func (Client) DecodeFromPtr(p Ptr) Client {
	return p.Interface().Client()
}

var _ TypeParam[Client] = Client{}

// isResolve reports whether ch has been resolved.
// The caller must be holding onto ch.mu.
func (ch *clientHook) isResolved() bool {
	select {
	case <-ch.resolved:
		return true
	default:
		return false
	}
}

var clientLeakFunc func(string)

// SetClientLeakFunc sets a callback for reporting Clients that went
// out of scope without being released.  The callback is not guaranteed
// to be called and must be safe to call concurrently from multiple
// goroutines.  The exact format of the message is unspecified.
//
// SetClientLeakFunc must not be called after any calls to NewClient or
// NewPromisedClient.
func SetClientLeakFunc(f func(msg string)) {
	clientLeakFunc = f
}

func (c Client) setFinalizer() {
	runtime.SetFinalizer(c.client, finalizeClient)
}

func finalizeClient(c *client) {
	// Since there are no other references to c, then we don't have to
	// acquire the mutex to read.
	if c.released {
		return
	}

	var fname string
	switch c.creatorFunc {
	case 1:
		fname = "NewClient"
	case 2:
		fname = "NewPromisedClient"
	case 3:
		fname = "AddRef"
	default:
		fname = "<???>"
	}
	var msg string
	if c.creatorFile == "" {
		msg = fmt.Sprintf("leaked client created by %s", fname)
	} else {
		msg = fmt.Sprintf("leaked client created by %s on %s:%d", fname, c.creatorFile, c.creatorLine)
	}

	// finalizeClient will only be called if clientLeakFunc != nil.
	go clientLeakFunc(msg)
}

// A ClientPromise resolves the identity of a client created by NewPromisedClient.
type ClientPromise struct {
	h *clientHook
}

func (cp *ClientPromise) Reject(err error) {
	cp.Fulfill(ErrorClient(err))
}

// Fulfill resolves the client promise to c.  After Fulfill returns,
// then all future calls to the client created by NewPromisedClient will
// be sent to c.  It is guaranteed that the hook passed to
// NewPromisedClient will be shut down after Fulfill returns, but the
// hook may have been shut down earlier if the client ran out of
// references.
func (cp *ClientPromise) Fulfill(c Client) {
	cp.fulfill(c)
	cp.shutdown()
}

// shutdown waits for all outstanding calls on the hook to complete and
// references to be dropped, and then shuts down the hook. The caller
// must have previously invoked cp.fulfill().
func (cp *ClientPromise) shutdown() {
	<-cp.h.done
	cp.h.Shutdown()
}

// fulfill is like Fulfill, except that it does not wait for outsanding calls
// to return answers or shut down the underlying hook.
func (cp *ClientPromise) fulfill(c Client) {
	// Obtain next client hook.
	var rh *clientHook
	if (c != Client{}) {
		c.mu.Lock()
		if c.released {
			c.mu.Unlock()
			panic("ClientPromise.Fulfill with a released client")
		}
		// TODO(maybe): c.h = resolveHook(c.h)
		rh = c.h
		c.mu.Unlock()
	}

	// Mark hook as resolved.
	cp.h.mu.Lock()
	if cp.h.isResolved() {
		cp.h.mu.Unlock()
		panic("ClientPromise.Fulfill called more than once")
	}
	cp.h.resolvedHook = rh
	close(cp.h.resolved)
	refs := cp.h.refs
	cp.h.refs = 0
	if refs == 0 {
		cp.h.mu.Unlock()
		return
	}

	// Client still had references, so we're responsible for shutting it down.
	if cp.h.calls == 0 {
		close(cp.h.done)
	}
	rh = resolveHook(cp.h) // swaps mutex on cp.h for mutex on rh
	if rh != nil {
		rh.refs += refs
		rh.mu.Unlock()
	}
}

// A WeakClient is a weak reference to a capability: it refers to a
// capability without preventing it from being shut down.  The zero
// value is a null reference.
type WeakClient struct {
	h *clientHook
}

// AddRef creates a new Client that refers to the same capability as c
// as long as the capability hasn't already been shut down.
func (wc *WeakClient) AddRef() (c Client, ok bool) {
	if wc == nil {
		return Client{}, true
	}
	if wc.h == nil {
		return Client{}, true
	}
	wc.h.mu.Lock()
	wc.h = resolveHook(wc.h)
	if wc.h == nil {
		return Client{}, true
	}
	if wc.h.refs == 0 {
		wc.h.mu.Unlock()
		return Client{}, false
	}
	wc.h.refs++
	wc.h.mu.Unlock()
	c = Client{client: &client{h: wc.h}}
	if clientLeakFunc != nil {
		c.creatorFunc = 3
		_, c.creatorFile, c.creatorLine, _ = runtime.Caller(1)
		c.setFinalizer()
	}
	return c, true
}

// A ClientHook represents a Cap'n Proto capability.  Application code
// should not pass around ClientHooks; applications should pass around
// Clients.  A ClientHook must be safe to use from multiple goroutines.
//
// Calls must be delivered to the capability in the order they are made.
// This guarantee is based on the concept of a capability
// acknowledging delivery of a call: this is specific to an
// implementation of ClientHook.  A type that implements ClientHook
// must guarantee that if foo() then bar() is called on a client, then
// the capability acknowledging foo() happens before the capability
// observing bar().
type ClientHook interface {
	// Send allocates space for parameters, calls s.PlaceArgs to fill out
	// the arguments, then starts executing a method, returning an answer
	// that will hold the result.  The hook must call s.PlaceArgs at most
	// once, and if it does call s.PlaceArgs, it must return before Send
	// returns.  The caller must call the returned release function when
	// it no longer needs the answer's data.
	//
	// Send is typically used when application code is making a call.
	Send(ctx context.Context, s Send) (*Answer, ReleaseFunc)

	// Recv starts executing a method with the referenced arguments
	// and places the result in a message controlled by the caller.
	// The hook will call r.ReleaseArgs when it no longer needs to
	// reference the parameters and use r.Returner to complete the method
	// call.  If Recv does not call r.Returner.Return before it returns,
	// then it must return a non-nil PipelineCaller.
	//
	// Recv is typically used when the RPC system has received a call.
	Recv(ctx context.Context, r Recv) PipelineCaller

	// Brand returns an implementation-specific value.  This can be used
	// to introspect and identify kinds of clients.
	Brand() Brand

	// Shutdown releases any resources associated with this capability.
	// The behavior of calling any methods on the receiver after calling
	// Shutdown is undefined.  It is expected for the ClientHook to reject
	// any outstanding call futures.
	Shutdown()
}

// Send is the input to ClientHook.Send.
type Send struct {
	// Method must have InterfaceID and MethodID filled in.
	Method Method

	// PlaceArgs is a function that will be called at most once before Send
	// returns to populate the arguments for the RPC.  PlaceArgs may be nil.
	PlaceArgs func(Struct) error

	// ArgsSize specifies the size of the struct to pass to PlaceArgs.
	ArgsSize ObjectSize
}

// Recv is the input to ClientHook.Recv.
type Recv struct {
	// Method must have InterfaceID and MethodID filled in.
	Method Method

	// Args is the set of arguments for the RPC.
	Args Struct

	// ReleaseArgs is called after Args is no longer referenced.
	// Must not be nil. If called more than once, subsequent calls
	// must silently no-op.
	ReleaseArgs ReleaseFunc

	// Returner manages the results.
	Returner Returner
}

// AllocResults allocates a result struct.  It is the same as calling
// r.Returner.AllocResults(sz).
func (r Recv) AllocResults(sz ObjectSize) (Struct, error) {
	return r.Returner.AllocResults(sz)
}

// Return ends the method call successfully, releasing the arguments.
func (r Recv) Return() {
	r.ReleaseArgs()
	r.Returner.Return(nil)
}

// Reject ends the method call with an error, releasing the arguments.
func (r Recv) Reject(e error) {
	if e == nil {
		panic("Reject(nil)")
	}
	r.ReleaseArgs()
	r.Returner.Return(e)
}

// A Returner allocates and sends the results from a received
// capability method call.
type Returner interface {
	// AllocResults allocates the results struct that will be sent using
	// Return.  It can be called at most once, and only before calling
	// Return.  The struct returned by AllocResults cannot be used after
	// Return is called.
	AllocResults(sz ObjectSize) (Struct, error)

	// Return resolves the method call successfully if e is nil, or failure
	// otherwise.  Return must be called once.
	//
	// Return must wait for all ongoing pipelined calls to be delivered,
	// and after it returns, no new calls can be sent to the PipelineCaller
	// returned from Recv.
	Return(e error)
}

// A ReleaseFunc tells the RPC system that a parameter or result struct
// is no longer in use and may be reclaimed.  After the first call,
// subsequent calls to a ReleaseFunc do nothing.  A ReleaseFunc should
// not be called concurrently.
type ReleaseFunc func()

// A Method identifies a method along with an optional human-readable
// description of the method.
type Method struct {
	InterfaceID uint64
	MethodID    uint16

	// Canonical name of the interface.  May be empty.
	InterfaceName string
	// Method name as it appears in the schema.  May be empty.
	MethodName string
}

// String returns a formatted string containing the interface name or
// the method name if present, otherwise it uses the raw IDs.
// This is suitable for use in error messages and logs.
func (m *Method) String() string {
	buf := make([]byte, 0, 128)
	if m.InterfaceName == "" {
		buf = append(buf, '@', '0', 'x')
		buf = strconv.AppendUint(buf, m.InterfaceID, 16)
	} else {
		buf = append(buf, m.InterfaceName...)
	}
	buf = append(buf, '.')
	if m.MethodName == "" {
		buf = append(buf, '@')
		buf = strconv.AppendUint(buf, uint64(m.MethodID), 10)
	} else {
		buf = append(buf, m.MethodName...)
	}
	return string(buf)
}

type errorClient struct {
	e error
}

// ErrorClient returns a Client that always returns error e.
// An ErrorClient does not need to be released: it is a sentinel like a
// nil Client.
//
// The returned client's State() method returns a State with its
// Brand.Value set to e.
func ErrorClient(e error) Client {
	if e == nil {
		panic("ErrorClient(nil)")
	}

	// Avoid NewClient because it can set a finalizer.
	h := &clientHook{
		ClientHook: errorClient{e},
		done:       make(chan struct{}),
		refs:       1,
		resolved:   closedSignal,
		metadata:   *NewMetadata(),
	}
	h.resolvedHook = h
	return Client{client: &client{h: h}}
}

func (ec errorClient) Send(_ context.Context, s Send) (*Answer, ReleaseFunc) {
	return ErrorAnswer(s.Method, ec.e), func() {}
}

func (ec errorClient) Recv(_ context.Context, r Recv) PipelineCaller {
	r.Reject(ec.e)
	return nil
}

func (ec errorClient) Brand() Brand {
	return Brand{Value: ec.e}
}

func (ec errorClient) Shutdown() {
}

var closedSignal = make(chan struct{})
