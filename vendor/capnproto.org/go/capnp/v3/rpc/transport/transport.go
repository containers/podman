// Package transport defines an interface for sending and receiving rpc messages.
package transport

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	capnp "capnproto.org/go/capnp/v3"
	rpccp "capnproto.org/go/capnp/v3/std/capnp/rpc"
)

// A Transport sends and receives Cap'n Proto RPC messages to and from
// another vat.
//
// It is safe to call NewMessage and its returned functions concurrently
// with RecvMessage.
type Transport interface {
	// NewMessage allocates a new message to be sent over the transport.
	// The caller must call the release function when it no longer needs
	// to reference the message.  Before releasing the message, send may be
	// called at most once to send the mssage, taking its cancelation and
	// deadline from ctx.
	//
	// Messages returned by NewMessage must have a nil CapTable.
	// When the returned ReleaseFunc is called, any clients in the message's
	// CapTable will be released.
	//
	// The Arena in the returned message should be fast at allocating new
	// segments.  The returned ReleaseFunc MUST be safe to call concurrently
	// with subsequent calls to NewMessage.
	NewMessage(ctx context.Context) (_ rpccp.Message, send func() error, _ capnp.ReleaseFunc, _ error)

	// RecvMessage receives the next message sent from the remote vat.
	// The returned message is only valid until the release function is
	// called or Close is called.  The release function may be called
	// concurrently with RecvMessage or with any other release function
	// returned by RecvMessage.
	//
	// Messages returned by RecvMessage must have a nil CapTable.
	// When the returned ReleaseFunc is called, any clients in the message's
	// CapTable will be released.
	//
	// The Arena in the returned message should not fetch segments lazily;
	// the Arena should be fast to access other segments.
	RecvMessage(ctx context.Context) (rpccp.Message, capnp.ReleaseFunc, error)

	// Close releases any resources associated with the transport.  All
	// messages created with NewMessage must be released before calling
	// Close.  It is not safe to call Close concurrently with any other
	// operations on the transport.
	Close() error
}

// A Codec is responsible for encoding and decoding messages from
// a single logical stream.
type Codec interface {
	Encode(context.Context, *capnp.Message) error
	Decode(context.Context) (*capnp.Message, error)
	SetPartialWriteTimeout(time.Duration)
	Close() error
}

// A transport serializes and deserializes Cap'n Proto using a Codec.
// It adds no buffering beyond what is provided by the underlying
// byte transfer mechanism.
type transport struct {
	c      Codec
	closed bool
	err    errorValue
}

// New creates a new transport that uses the supplied codec
// to read and write messages across the wire.
func New(c Codec) Transport { return &transport{c: c} }

// NewStream creates a new transport that reads and writes to rwc.
// Closing the transport will close rwc.
//
// If rwc has SetReadDeadline or SetWriteDeadline methods, they will be
// used to handle Context cancellation and deadlines.  If rwc does not
// have these methods, then rwc.Close must be safe to call concurrently
// with rwc.Read.  Notably, this is not true of *os.File before Go 1.9
// (see https://golang.org/issue/7970).
func NewStream(rwc io.ReadWriteCloser) Transport {
	return New(newStreamCodec(rwc, basicEncoding{}))
}

// NewPackedStream creates a new transport that uses a packed
// encoding.
//
// See:  NewStream.
func NewPackedStream(rwc io.ReadWriteCloser) Transport {
	return New(newStreamCodec(rwc, packedEncoding{}))
}

// NewMessage allocates a new message to be sent.
//
// It is safe to call NewMessage concurrently with RecvMessage.
func (s *transport) NewMessage(ctx context.Context) (_ rpccp.Message, send func() error, release capnp.ReleaseFunc, _ error) {
	// Check if stream is broken
	if err := s.err.Load(); err != nil {
		return rpccp.Message{}, nil, nil, err
	}

	// TODO(soon): reuse memory
	msg, seg, err := capnp.NewMessage(capnp.MultiSegment(nil))
	if err != nil {
		err = transporterr.Annotate(fmt.Errorf("new message: %w", err), "stream transport")
		return rpccp.Message{}, nil, nil, err
	}
	rmsg, err := rpccp.NewRootMessage(seg)
	if err != nil {
		err = transporterr.Annotate(fmt.Errorf("new message: %w", err), "stream transport")
		return rpccp.Message{}, nil, nil, err
	}

	send = func() error {
		// context expired?
		if err := ctx.Err(); err != nil {
			return transporterr.Annotate(fmt.Errorf("send: %w", ctx.Err()), "stream transport")
		}

		// stream error?
		if err := s.err.Load(); err != nil {
			return err
		}

		// ok, go!
		if err = s.c.Encode(ctx, msg); err != nil {
			if _, ok := err.(partialWriteError); ok {
				s.err.Set(transporterr.
					Disconnectedf("broken due to partial write").
					Annotate("", "stream transport"))
			}

			err = transporterr.Annotate(fmt.Errorf("send: %w", err), "stream transport")
		}

		return err
	}

	return rmsg, send, func() { msg.Reset(nil) }, nil
}

// SetPartialWriteTimeout sets the timeout for completing the
// transmission of a partially sent message after the send is cancelled
// or interrupted for any future sends.  If not set, a reasonable
// non-zero value is used.
//
// Setting a shorter timeout may free up resources faster in the case of
// an unresponsive remote peer, but may also make the transport respond
// too aggressively to bursts of latency.
func (s *transport) SetPartialWriteTimeout(d time.Duration) {
	s.c.SetPartialWriteTimeout(d)
}

// RecvMessage reads the next message from the underlying reader.
//
// It is safe to call RecvMessage concurrently with NewMessage.
func (s *transport) RecvMessage(ctx context.Context) (rpccp.Message, capnp.ReleaseFunc, error) {
	if err := s.err.Load(); err != nil {
		return rpccp.Message{}, nil, err
	}

	msg, err := s.c.Decode(ctx)
	if err != nil {
		err = transporterr.Annotate(fmt.Errorf("receive: %w", err), "stream transport")
		return rpccp.Message{}, nil, err
	}
	rmsg, err := rpccp.ReadRootMessage(msg)
	if err != nil {
		err = transporterr.Annotate(fmt.Errorf("receive: %w", err), "stream transport")
		return rpccp.Message{}, nil, err
	}
	return rmsg, func() { msg.Reset(nil) }, nil
}

// Close closes the underlying ReadWriteCloser.  It is not safe to call
// Close concurrently with any other operations on the transport.
func (s *transport) Close() error {
	if s.closed {
		return transporterr.Disconnectedf("already closed").Annotate("", "stream transport")
	}
	s.closed = true
	err := s.c.Close()
	if err != nil {
		return transporterr.Annotate(fmt.Errorf("close: %w", err), "stream transport")
	}
	return nil
}

type streamCodec struct {
	r   *ctxReader
	dec *capnp.Decoder

	wc  *ctxWriteCloser
	enc *capnp.Encoder
}

func newStreamCodec(rwc io.ReadWriteCloser, f streamEncoding) *streamCodec {
	c := &streamCodec{
		r: &ctxReader{Reader: rwc},
		wc: &ctxWriteCloser{
			WriteCloser:         rwc,
			partialWriteTimeout: 30 * time.Second,
		},
	}

	c.dec = f.NewDecoder(c.r)
	c.enc = f.NewEncoder(c.wc)

	return c
}

func (c *streamCodec) Encode(ctx context.Context, m *capnp.Message) error {
	c.wc.setWriteContext(ctx)
	return c.enc.Encode(m)
}

func (c *streamCodec) Decode(ctx context.Context) (*capnp.Message, error) {
	c.r.setReadContext(ctx)
	return c.dec.Decode()
}

func (c *streamCodec) SetPartialWriteTimeout(d time.Duration) {
	c.wc.partialWriteTimeout = d
}

func (c streamCodec) Close() error {
	defer c.r.wait()

	return c.wc.Close()
}

type streamEncoding interface {
	NewEncoder(io.Writer) *capnp.Encoder
	NewDecoder(io.Reader) *capnp.Decoder
}

type basicEncoding struct{}

func (basicEncoding) NewEncoder(w io.Writer) *capnp.Encoder { return capnp.NewEncoder(w) }
func (basicEncoding) NewDecoder(r io.Reader) *capnp.Decoder { return capnp.NewDecoder(r) }

type packedEncoding struct{}

func (packedEncoding) NewEncoder(w io.Writer) *capnp.Encoder { return capnp.NewPackedEncoder(w) }
func (packedEncoding) NewDecoder(r io.Reader) *capnp.Decoder { return capnp.NewPackedDecoder(r) }

// ctxReader adds timeouts and cancellation to a reader.
type ctxReader struct {
	io.Reader
	ctx context.Context // set to change Context

	// internal state
	result chan readResult
	pos, n int
	err    error
	buf    [1024]byte
}

type readResult struct {
	n   int
	err error
}

func (cr *ctxReader) setReadContext(ctx context.Context) { cr.ctx = ctx }

// Read reads into p.  It makes a best effort to respect the Done signal
// in cr.ctx.
func (cr *ctxReader) Read(p []byte) (int, error) {
	if cr.pos < cr.n {
		// Buffered from previous read.
		n := copy(p, cr.buf[cr.pos:cr.n])
		cr.pos += n
		if cr.pos == cr.n && cr.err != nil {
			err := cr.err
			cr.err = nil
			return n, err
		}
		return n, nil
	}
	if cr.result != nil {
		// Read in progress.
		select {
		case r := <-cr.result:
			cr.result = nil
			cr.n = r.n
			cr.pos = copy(p, cr.buf[:cr.n])
			if cr.pos == cr.n && r.err != nil {
				return cr.pos, r.err
			}
			cr.err = r.err
			return cr.pos, nil
		case <-cr.ctx.Done():
			return 0, cr.ctx.Err()
		}
	}
	// Check for early cancel.
	select {
	case <-cr.ctx.Done():
		return 0, cr.ctx.Err()
	default:
	}
	// Query timeout support.
	rd, ok := cr.Reader.(interface {
		SetReadDeadline(time.Time) error
	})
	if !ok {
		return cr.leakyRead(p)
	}
	if err := rd.SetReadDeadline(time.Now()); err != nil {
		return cr.leakyRead(p)
	}
	// Start separate goroutine to wait on Context.Done.
	if d, ok := cr.ctx.Deadline(); ok {
		rd.SetReadDeadline(d)
	} else {
		rd.SetReadDeadline(time.Time{})
	}
	readDone := make(chan struct{})
	listenDone := make(chan struct{})
	go func() {
		defer close(listenDone)
		select {
		case <-cr.ctx.Done():
			rd.SetReadDeadline(time.Now()) // interrupt read
		case <-readDone:
		}
	}()
	n, err := cr.Reader.Read(p)
	close(readDone)
	<-listenDone
	return n, err
}

// leakyRead reads from the underlying reader in a separate goroutine.
// If the Context is Done before the read completes, then the goroutine
// will stay alive until cr.wait() is called.
func (cr *ctxReader) leakyRead(p []byte) (int, error) {
	cr.result = make(chan readResult)
	max := len(p)
	if max > len(cr.buf) {
		max = len(cr.buf)
	}
	go func() {
		n, err := cr.Reader.Read(cr.buf[:max])
		cr.result <- readResult{n, err}
	}()
	select {
	case r := <-cr.result:
		cr.result = nil
		copy(p, cr.buf[:r.n])
		return r.n, r.err
	case <-cr.ctx.Done():
		return 0, cr.ctx.Err()
	}
}

// wait until any goroutine started by leakyRead finishes.
func (cr *ctxReader) wait() {
	if cr.result == nil {
		return
	}
	r := <-cr.result
	cr.result = nil
	cr.pos, cr.n = 0, r.n
	cr.err = r.err
}

type ctxWriteCloser struct {
	io.WriteCloser
	ctx                 context.Context
	partialWriteTimeout time.Duration
}

// Write bytes to a writer while making a best effort to
// respect the Done signal of the Context.  However, if allowPartial is
// false, then once any bytes have been written to w, writeCtx will
// ignore the Done signal to avoid partial writes.
func (wc *ctxWriteCloser) Write(b []byte) (int, error) {
	n, err := wc.write(b)
	if n > 0 && n < len(b) {
		err = partialWriteError{err}
	}

	return n, err
}

func (wc *ctxWriteCloser) setWriteContext(ctx context.Context) { wc.ctx = ctx }

func (wc *ctxWriteCloser) write(b []byte) (int, error) {
	select {
	case <-wc.ctx.Done():
		// Early cancel.
		return 0, wc.ctx.Err()
	default:
	}
	// Check for timeout support.
	wd, ok := wc.WriteCloser.(interface {
		SetWriteDeadline(time.Time) error
	})
	if !ok {
		return wc.WriteCloser.Write(b)
	}
	if err := wd.SetWriteDeadline(time.Now()); err != nil {
		return wc.WriteCloser.Write(b)
	}
	// Start separate goroutine to wait on Context.Done.
	if d, ok := wc.ctx.Deadline(); ok {
		wd.SetWriteDeadline(d)
	} else {
		wd.SetWriteDeadline(time.Time{})
	}
	writeDone := make(chan struct{})
	listenDone := make(chan struct{})
	go func() {
		defer close(listenDone)
		select {
		case <-wc.ctx.Done():
			wd.SetWriteDeadline(time.Now()) // interrupt write
		case <-writeDone:
		}
	}()
	n, err := wc.WriteCloser.Write(b)
	close(writeDone)
	<-listenDone
	if wc.partialWriteTimeout <= 0 || n == 0 || !isTimeout(err) {
		return n, err
	}
	// Data has been written.  Block with extra partial timeout, since
	// partial writes are guaranteed protocol violations.
	wd.SetWriteDeadline(time.Now().Add(wc.partialWriteTimeout))
	nn, err := wc.WriteCloser.Write(b[n:])
	return n + nn, err
}

func isTimeout(e error) bool {
	te, ok := e.(interface {
		Timeout() bool
	})
	return ok && te.Timeout()
}

type partialWriteError struct{ error }

type errorValue atomic.Value

func (ev *errorValue) Load() error {
	if err := (*atomic.Value)(ev).Load(); err != nil {
		return err.(error)
	}

	return nil
}

func (ev *errorValue) Set(err error) {
	(*atomic.Value)(ev).Store(err)
}
