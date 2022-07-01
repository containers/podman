package server

import (
	"context"
	"sync"

	"capnproto.org/go/capnp/v3"
)

// answerQueue is a queue of method calls to make after an earlier
// method call finishes.  The queue is unbounded; it is the caller's
// responsibility to manage/impose backpressure.
//
// An answerQueue can be in one of three states:
//
//	1) Queueing.  Incoming method calls will be added to the queue.
//	2) Draining, entered by calling fulfill or reject.  Queued method
//	   calls will be delivered in sequence, and new incoming method calls
//	   will block until the answerQueue enters the Drained state.
//	3) Drained, entered once all queued methods have been delivered.
//	   Incoming methods are passthrough.
type answerQueue struct {
	method   capnp.Method
	draining chan struct{} // closed while exiting queueing state

	mu    sync.Mutex
	q     []qent // non-nil while queueing
	bases []base // set when drain starts. len(bases) >= 1
}

// qent is a single entry in an answerQueue.
type qent struct {
	ctx   context.Context
	basis int // index in bases
	path  clientPath
	capnp.Recv
}

// base is a message target derived from applying a qent.
type base struct {
	ready chan struct{} // closed after recv is assigned
	recv  func(context.Context, []capnp.PipelineOp, capnp.Recv) capnp.PipelineCaller
}

// newAnswerQueue creates a new answer queue.
func newAnswerQueue(m capnp.Method) *answerQueue {
	return &answerQueue{
		method: m,
		// N.B. since q == nil denotes the draining state,
		// we do have to allocate something here, even though
		// the queue is an empty slice.
		q:        make([]qent, 0),
		draining: make(chan struct{}),
	}
}

// fulfill empties the queue, delivering the method calls on the given
// struct.  After fulfill returns, pipeline calls will be immediately
// delivered instead of being queued.
func (aq *answerQueue) fulfill(s capnp.Struct) {
	// Enter draining state.
	aq.mu.Lock()
	q := aq.q
	aq.q = nil
	aq.bases = make([]base, len(q)+1)
	ready := make(chan struct{}) // TODO(soon): use more fine-grained signals
	defer close(ready)
	for i := range aq.bases {
		aq.bases[i].ready = ready
	}
	aq.bases[0].recv = capnp.ImmediateAnswer(aq.method, s).PipelineRecv
	close(aq.draining)
	aq.mu.Unlock()

	// Drain queue.
	embargoes := make([]returnEmbargoer, len(q))
	for i := range q {
		ent := &q[i]
		recv := aq.bases[ent.basis].recv
		embargoes[i].alloc = ent.Returner
		embargoes[i].returned = make(chan struct{})
		embargoes[i].pcall = recv(ent.ctx, ent.path.transform(), capnp.Recv{
			Method:      ent.Method,
			Args:        ent.Args,
			ReleaseArgs: ent.ReleaseArgs,
			Returner:    &embargoes[i],
		})
		aq.bases[i+1].recv = (&embargoes[i]).recv
	}

	// Asynchronously deliver returns.
	// fulfill just needs to guarantee delivery, not completion.
	// TODO(maybe): avoid goroutine if return already happened
	for i := range embargoes {
		go func(e *returnEmbargoer, ret capnp.Returner) {
			<-e.returned
			e.mu.Lock()
			err := e.err
			e.mu.Unlock()
			ret.Return(err)
		}(&embargoes[i], q[i].Returner)
	}
}

// reject empties the queue, returning errors on all the method calls.
func (aq *answerQueue) reject(e error) {
	if e == nil {
		panic("answerQueue.reject(nil)")
	}

	// Enter draining state.
	aq.mu.Lock()
	q := aq.q
	aq.q = nil
	aq.bases = make([]base, len(q)+1)
	ready := make(chan struct{})
	close(ready)
	for i := range aq.bases {
		b := &aq.bases[i]
		b.ready = ready
		b.recv = func(_ context.Context, _ []capnp.PipelineOp, r capnp.Recv) capnp.PipelineCaller {
			r.Reject(e) // TODO(soon): attach pipelined method info
			return nil
		}
	}
	close(aq.draining)
	aq.mu.Unlock()

	// Drain queue by rejecting.
	for i := range q {
		q[i].Reject(e) // TODO(soon): attach pipelined method info
	}
}

func (aq *answerQueue) PipelineRecv(ctx context.Context, transform []capnp.PipelineOp, r capnp.Recv) capnp.PipelineCaller {
	return queueCaller{aq, 0}.PipelineRecv(ctx, transform, r)
}

func (aq *answerQueue) PipelineSend(ctx context.Context, transform []capnp.PipelineOp, r capnp.Send) (*capnp.Answer, capnp.ReleaseFunc) {
	return queueCaller{aq, 0}.PipelineSend(ctx, transform, r)
}

// queueCaller is a client that enqueues calls to an answerQueue.
type queueCaller struct {
	aq    *answerQueue
	basis int
}

func (qc queueCaller) PipelineRecv(ctx context.Context, transform []capnp.PipelineOp, r capnp.Recv) capnp.PipelineCaller {
	qc.aq.mu.Lock()
	switch {
	case len(qc.aq.bases) > 0:
		// Draining/drained.
		qc.aq.mu.Unlock()
		b := &qc.aq.bases[qc.basis]
		select {
		case <-b.ready:
		case <-ctx.Done():
			r.Reject(ctx.Err())
			return nil
		}
		return b.recv(ctx, transform, r)
	default:
		// Enqueue.
		qc.aq.q = append(qc.aq.q, qent{
			ctx:   ctx,
			basis: qc.basis,
			path:  clientPathFromTransform(transform),
			Recv:  r,
		})
		basis := len(qc.aq.q) - 1
		qc.aq.mu.Unlock()
		return queueCaller{aq: qc.aq, basis: basis}
	}
}

func (qc queueCaller) PipelineSend(ctx context.Context, transform []capnp.PipelineOp, s capnp.Send) (*capnp.Answer, capnp.ReleaseFunc) {
	ret := new(structReturner)
	r := capnp.Recv{
		Method:   s.Method,
		Returner: ret,
	}
	if s.PlaceArgs != nil {
		var err error
		r.Args, err = newBlankStruct(s.ArgsSize)
		if err != nil {
			return capnp.ErrorAnswer(s.Method, err), func() {}
		}
		if err = s.PlaceArgs(r.Args); err != nil {
			return capnp.ErrorAnswer(s.Method, err), func() {}
		}
		r.ReleaseArgs = func() {
			r.Args.Message().Reset(nil)
		}
	} else {
		r.ReleaseArgs = func() {}
	}
	pcall := qc.PipelineRecv(ctx, transform, r)
	return ret.answer(s.Method, pcall)
}

// A structReturner implements Returner by allocating an in-memory
// message.  It is safe to use from multiple goroutines.  The zero value
// is a Returner in its initial state.
type structReturner struct {
	mu      sync.Mutex     // guards all fields
	p       *capnp.Promise // assigned at most once
	alloced bool

	returned bool         // indicates whether the below fields are filled in
	result   capnp.Struct // assigned at most once
	err      error        // assigned at most once
}

func (sr *structReturner) AllocResults(sz capnp.ObjectSize) (capnp.Struct, error) {
	defer sr.mu.Unlock()
	sr.mu.Lock()
	if sr.alloced {
		return capnp.Struct{}, newError("multiple calls to AllocResults")
	}
	sr.alloced = true
	s, err := newBlankStruct(sz)
	if err != nil {
		return capnp.Struct{}, errorf("alloc results: %v", err)
	}
	sr.result = s
	return s, nil
}

func (sr *structReturner) Return(e error) {
	sr.mu.Lock()
	if sr.returned {
		sr.mu.Unlock()
		panic("structReturner.Return called twice")
	}
	sr.returned = true
	if e == nil {
		sr.mu.Unlock()
		if sr.p != nil {
			sr.p.Fulfill(sr.result.ToPtr())
		}
	} else {
		msg := sr.result.Message()
		sr.result = capnp.Struct{}
		sr.err = e
		sr.mu.Unlock()
		if msg != nil {
			msg.Reset(nil)
		}
		if sr.p != nil {
			sr.p.Reject(e)
		}
	}
}

// answer returns an Answer that will be resolved when Return is called.
// answer must only be called once per structReturner.
func (sr *structReturner) answer(m capnp.Method, pcall capnp.PipelineCaller) (*capnp.Answer, capnp.ReleaseFunc) {
	defer sr.mu.Unlock()
	sr.mu.Lock()
	if sr.p != nil {
		panic("structReturner.answer called multiple times")
	}
	if sr.returned {
		if sr.err != nil {
			return capnp.ErrorAnswer(m, sr.err), func() {}
		}
		return capnp.ImmediateAnswer(m, sr.result), func() {
			sr.mu.Lock()
			msg := sr.result.Message()
			sr.result = capnp.Struct{}
			sr.mu.Unlock()
			if msg != nil {
				msg.Reset(nil)
			}
		}
	}
	sr.p = capnp.NewPromise(m, pcall)
	ans := sr.p.Answer()
	return ans, func() {
		<-ans.Done()
		sr.mu.Lock()
		msg := sr.result.Message()
		sr.result = capnp.Struct{}
		sr.mu.Unlock()
		if msg != nil {
			msg.Reset(nil)
		}
	}
}

// returnEmbargoer collects a return result without sending it anywhere,
// while allowing pipeline calls.
type returnEmbargoer struct {
	alloc    resultsAllocer
	pcall    capnp.PipelineCaller
	calls    sync.WaitGroup
	returned chan struct{} // closed after result and err finalized

	mu     sync.Mutex
	result capnp.Struct
	err    error
}

func (re *returnEmbargoer) AllocResults(sz capnp.ObjectSize) (capnp.Struct, error) {
	s, err := re.alloc.AllocResults(sz)
	re.mu.Lock()
	re.result = s
	re.mu.Unlock()
	return s, err
}

func (re *returnEmbargoer) Return(e error) {
	re.mu.Lock()
	re.err = e
	close(re.returned)
	re.mu.Unlock()
	re.calls.Wait()
}

// recv starts a pipelined call on the answer, either using the
// PipelineCaller or a client in the returned result.
func (re *returnEmbargoer) recv(ctx context.Context, transform []capnp.PipelineOp, r capnp.Recv) capnp.PipelineCaller {
	re.mu.Lock()
	select {
	case <-re.returned:
		re.mu.Unlock()
		if re.err != nil {
			r.Reject(re.err)
			return nil
		}
		return capnp.ImmediateAnswer(r.Method, re.result).PipelineRecv(ctx, transform, r)
	default:
		re.calls.Add(1)
		defer re.calls.Done()
		re.mu.Unlock()
		return re.pcall.PipelineRecv(ctx, transform, r)
	}
}

// clientPath is an encoded version of a list of pipeline operations.
// It is suitable as a map key.
//
// It specifically ignores default values, because a capability can't have a
// default value other than null.
type clientPath string

func clientPathFromTransform(ops []capnp.PipelineOp) clientPath {
	buf := make([]byte, 0, len(ops)*2)
	for i := range ops {
		f := ops[i].Field
		buf = append(buf, byte(f&0x00ff), byte(f&0xff00>>8))
	}
	return clientPath(buf)
}

func (cp clientPath) transform() []capnp.PipelineOp {
	ops := make([]capnp.PipelineOp, len(cp)/2)
	for i := range ops {
		ops[i].Field = uint16(cp[i*2]) | uint16(cp[i*2+1])<<8
	}
	return ops
}
