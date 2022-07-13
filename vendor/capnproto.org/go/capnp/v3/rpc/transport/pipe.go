package transport

import (
	"context"
	"fmt"
	"io"
	"time"

	capnp "capnproto.org/go/capnp/v3"
)

// NewPipe returns a pair of codecs which communicate over
// channels, copying messages at the channel boundary.
// bufSz is the size of the channel buffers.
func NewPipe(bufSz int) (c1, c2 Codec) {
	ch1 := make(chan *capnp.Message, bufSz)
	ch2 := make(chan *capnp.Message, bufSz)

	c1 = &pipe{
		send: ch1, recv: ch2,
	}

	c2 = &pipe{
		send: ch2, recv: ch1,
	}

	return
}

type pipe struct {
	send    chan<- *capnp.Message
	recv    <-chan *capnp.Message
	timeout <-chan time.Time
}

func (p *pipe) Encode(ctx context.Context, m *capnp.Message) (err error) {
	b, err := m.Marshal()
	if err != nil {
		return err
	}

	if m, err = capnp.Unmarshal(b); err != nil {
		return err
	}

	// send-channel may be closed
	defer func() {
		if v := recover(); v != nil {
			err = io.ErrClosedPipe
		}
	}()

	select {
	case p.send <- m:
		return nil
	case <-p.timeout:
		return fmt.Errorf("partial write timeout")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *pipe) Decode(ctx context.Context) (*capnp.Message, error) {
	select {
	case m, ok := <-p.recv:
		if !ok {
			return nil, io.ErrClosedPipe
		}

		return m, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *pipe) SetPartialWriteTimeout(d time.Duration) {
	p.timeout = time.After(d)
}

func (p *pipe) Close() error {
	close(p.send)
	return nil
}
