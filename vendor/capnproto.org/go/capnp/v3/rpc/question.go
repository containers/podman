package rpc

import (
	"context"

	"capnproto.org/go/capnp/v3"
	"capnproto.org/go/capnp/v3/internal/syncutil"
	rpccp "capnproto.org/go/capnp/v3/std/capnp/rpc"
)

// A questionID is an index into the questions table.
type questionID uint32

type question struct {
	c  *Conn
	id questionID

	bootstrapPromise *capnp.ClientPromise

	p       *capnp.Promise
	release capnp.ReleaseFunc // written before resolving p

	// Protected by c.mu:

	flags         questionFlags
	finishMsgSend chan struct{}        // closed after attempting to send the Finish message
	called        [][]capnp.PipelineOp // paths to called clients
}

// questionFlags is a bitmask of which events have occurred in a question's
// lifetime.
type questionFlags uint8

const (
	// finished is set when the question's Context has been canceled or
	// its Return message has been received.  The codepath that sets this
	// flag is responsible for sending the Finish message.
	finished questionFlags = 1 << iota

	// finishSent indicates whether the Finish message was sent
	// successfully.  It is only valid to query after finishMsgSend is
	// closed.
	finishSent
)

// newQuestion adds a new question to c's table.  The caller must be
// holding onto c.mu.
func (c *Conn) newQuestion(method capnp.Method) *question {
	q := &question{
		c:             c,
		id:            questionID(c.questionID.next()),
		release:       func() {},
		finishMsgSend: make(chan struct{}),
	}
	q.p = capnp.NewPromise(method, q) // TODO(someday): customize error message for bootstrap
	c.setAnswerQuestion(q.p.Answer(), q)
	if int(q.id) == len(c.questions) {
		c.questions = append(c.questions, q)
	} else {
		c.questions[q.id] = q
	}
	return q
}

func (c *Conn) getAnswerQuestion(ans *capnp.Answer) (*question, bool) {
	m := ans.Metadata()
	m.Lock()
	defer m.Unlock()
	q, ok := m.Get(questionKey{c})
	if !ok {
		return nil, false
	}
	return q.(*question), true
}

func (c *Conn) setAnswerQuestion(ans *capnp.Answer, q *question) {
	m := ans.Metadata()
	syncutil.With(m, func() {
		m.Put(questionKey{c}, q)
	})
}

type questionKey struct {
	conn *Conn
}

// handleCancel rejects the question's promise upon cancelation of its
// Context.
//
// The caller MUST NOT hold q.c.mu.
func (q *question) handleCancel(ctx context.Context) {
	var rejectErr error
	select {
	case <-ctx.Done():
		rejectErr = ctx.Err()
	case <-q.c.bgctx.Done():
		rejectErr = ExcClosed
	case <-q.p.Answer().Done():
		return
	}

	q.c.mu.Lock()
	defer q.c.mu.Unlock()

	// Promise already fulfilled?
	if q.flags&finished != 0 {
		return
	}
	q.flags |= finished
	q.release = func() {}

	q.c.sendMessage(q.c.bgctx, func(m rpccp.Message) error {
		fin, err := m.NewFinish()
		if err != nil {
			return err
		}
		fin.SetQuestionId(uint32(q.id))
		fin.SetReleaseResultCaps(true)
		return nil
	}, func(err error) {
		if err == nil {
			syncutil.With(&q.c.mu, func() { q.flags |= finishSent })
		} else if q.c.bgctx.Err() == nil {
			q.c.er.ReportError(rpcerr.Annotate(err, "send finish"))
		}
		close(q.finishMsgSend)

		q.p.Reject(rejectErr)
		if q.bootstrapPromise != nil {
			q.bootstrapPromise.Fulfill(q.p.Answer().Client())
			q.p.ReleaseClients()
		}
	})
}

func (q *question) PipelineSend(ctx context.Context, transform []capnp.PipelineOp, s capnp.Send) (*capnp.Answer, capnp.ReleaseFunc) {
	q.c.mu.Lock()
	defer q.c.mu.Unlock()

	if !q.c.startTask() {
		return capnp.ErrorAnswer(s.Method, ExcClosed), func() {}
	}
	defer q.c.tasks.Done()

	// Mark this transform as having been used for a call ASAP.
	// q's Return could be received while q2 is being sent.
	// Don't bother cleaning it up if the call fails because:
	// a) this may not have been the only call for the given transform,
	// b) the transform isn't guaranteed to be an import, and
	// c) the worst that happens is we trade bandwidth for code simplicity.
	q.mark(transform)
	q2 := q.c.newQuestion(s.Method)

	syncutil.Without(&q.c.mu, func() {
		// Send call message.
		q.c.sendMessage(ctx, func(m rpccp.Message) error {
			return q.c.newPipelineCallMessage(m, q.id, transform, q2.id, s)
		}, func(err error) {
			if err != nil {
				syncutil.With(&q.c.mu, func() {
					q.c.questions[q2.id] = nil
				})
				q2.p.Reject(rpcerr.Failedf("send message: %w", err))
				syncutil.With(&q.c.mu, func() {
					q.c.questionID.remove(uint32(q2.id))
				})
				return
			}

			q2.c.tasks.Add(1)
			go func() {
				defer q2.c.tasks.Done()
				q2.handleCancel(ctx)
			}()
		})
	})

	ans := q2.p.Answer()
	return ans, func() {
		<-ans.Done()
		q2.p.ReleaseClients()
		q2.release()
	}
}

// newPipelineCallMessage builds a Call message targeted to a promised answer..
//
// The caller MUST NOT hold c.mu.
func (c *Conn) newPipelineCallMessage(msg rpccp.Message, tgt questionID, transform []capnp.PipelineOp, qid questionID, s capnp.Send) error {
	call, err := msg.NewCall()
	if err != nil {
		return rpcerr.Failedf("build call message: %w", err)
	}
	call.SetQuestionId(uint32(qid))
	call.SetInterfaceId(s.Method.InterfaceID)
	call.SetMethodId(s.Method.MethodID)

	target, err := call.NewTarget()
	if err != nil {
		return rpcerr.Failedf("build call message: %w", err)
	}
	pa, err := target.NewPromisedAnswer()
	if err != nil {
		return rpcerr.Failedf("build call message: %w", err)
	}
	pa.SetQuestionId(uint32(tgt))
	oplist, err := pa.NewTransform(int32(len(transform)))
	if err != nil {
		return rpcerr.Failedf("build call message: %w", err)
	}
	for i, op := range transform {
		oplist.At(i).SetGetPointerField(op.Field)
	}

	payload, err := call.NewParams()
	if err != nil {
		return rpcerr.Failedf("build call message: %w", err)
	}
	args, err := capnp.NewStruct(payload.Segment(), s.ArgsSize)
	if err != nil {
		return rpcerr.Failedf("build call message: %w", err)
	}
	if err := payload.SetContent(args.ToPtr()); err != nil {
		return rpcerr.Failedf("build call message: %w", err)
	}

	if s.PlaceArgs == nil {
		return nil
	}
	m := args.Message()
	if err := s.PlaceArgs(args); err != nil {
		return rpcerr.Failedf("place arguments: %w", err)
	}
	clients := m.CapTable
	syncutil.With(&c.mu, func() {
		// TODO(soon): save param refs
		_, err = c.fillPayloadCapTable(payload, clients)
	})

	if err != nil {
		return rpcerr.Annotatef(err, "build call message")
	}

	return err
}

func (q *question) PipelineRecv(ctx context.Context, transform []capnp.PipelineOp, r capnp.Recv) capnp.PipelineCaller {
	ans, finish := q.PipelineSend(ctx, transform, capnp.Send{
		Method:   r.Method,
		ArgsSize: r.Args.Size(),
		PlaceArgs: func(s capnp.Struct) error {
			err := s.CopyFrom(r.Args)
			r.ReleaseArgs()
			return err
		},
	})
	r.ReleaseArgs()
	select {
	case <-ans.Done():
		returnAnswer(r.Returner, ans, finish)
		return nil
	default:
		go returnAnswer(r.Returner, ans, finish)
		return ans
	}
}

// mark adds the promised answer transform to the set of pipelined
// questions sent.  The caller must be holding onto q.c.mu.
func (q *question) mark(xform []capnp.PipelineOp) {
	for _, x := range q.called {
		if transformsEqual(x, xform) {
			// Already in set.
			return
		}
	}
	// Add a copy (don't retain default values).
	xform2 := make([]capnp.PipelineOp, len(xform))
	for i := range xform {
		xform2[i].Field = xform[i].Field
	}
	q.called = append(q.called, xform2)
}

func (q *question) Reject(err error) {
	if q != nil {
		if q.bootstrapPromise != nil {
			q.bootstrapPromise.Fulfill(capnp.ErrorClient(err))
		}

		if q.p != nil {
			q.p.Reject(err)
		}
	}
}

func transformsEqual(x1, x2 []capnp.PipelineOp) bool {
	if len(x1) != len(x2) {
		return false
	}
	for i := range x1 {
		if x1[i].Field != x2[i].Field {
			return false
		}
	}
	return true
}
