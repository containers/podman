package ctxio

import (
	"bufio"
	"context"
	"net"
	"time"
)

// Conn wraps net.Conn with context aware functionality.
type Conn struct {
	conn   net.Conn
	reader *bufio.Reader
}

// NewConn creates a new context aware Conn.
func NewConn(c net.Conn) *Conn {
	return &Conn{
		conn:   c,
		reader: bufio.NewReader(c),
	}
}

type ioret struct {
	n   int
	err error
}

type rret struct {
	val []byte
	err error
}

// aLongTimeAgo is a time in the past that indicates a connection should
// immediately time out.
var aLongTimeAgo = time.Unix(1, 0)

// Close releases the Conns resources.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// Write writes to the underlying connection.
// It is not safe for concurrent use with itself.
func (c *Conn) Write(ctx context.Context, buf []byte) (int, error) {
	// Enable immediate connection cancelation via context by using the context's
	// deadline and also setting a deadline in the past if/when the context is
	// canceled. This pattern courtesy of @acln from #networking on Gophers Slack.
	dl, _ := ctx.Deadline()
	if err := c.conn.SetWriteDeadline(dl); err != nil {
		return 0, err
	}

	ch := make(chan ioret, 1)
	go func() {
		n, err := c.conn.Write(buf)
		ch <- ioret{n, err}
	}()

	select {
	case <-ctx.Done():
		// Set deadline to unblock pending Write.
		if err := c.conn.SetWriteDeadline(aLongTimeAgo); err != nil {
			return 0, err
		}
		// Wait for goroutine to exit, throwing away the error.
		<-ch
		// Reset deadline again.
		if err := c.conn.SetWriteDeadline(time.Time{}); err != nil {
			return 0, err
		}
		return 0, ctx.Err()
	case ret := <-ch:
		return ret.n, ret.err
	}
}

// Read reads from the underlying connection.
// It is not safe for concurrent use with itself or ReadBytes.
func (c *Conn) Read(ctx context.Context, buf []byte) (int, error) {
	// Enable immediate connection cancelation via context by using the context's
	// deadline and also setting a deadline in the past if/when the context is
	// canceled. This pattern courtesy of @acln from #networking on Gophers Slack.
	dl, _ := ctx.Deadline()
	if err := c.conn.SetReadDeadline(dl); err != nil {
		return 0, err
	}

	ch := make(chan ioret, 1)
	go func() {
		n, err := c.conn.Read(buf)
		ch <- ioret{n, err}
	}()

	select {
	case <-ctx.Done():
		// Set deadline to unblock pending Read.
		if err := c.conn.SetReadDeadline(aLongTimeAgo); err != nil {
			return 0, err
		}
		// Wait for goroutine to exit, throwing away the error.
		<-ch
		// Reset deadline again.
		if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
			return 0, err
		}
		return 0, ctx.Err()
	case ret := <-ch:
		return ret.n, ret.err
	}
}

// ReadBytes reads from the connection until the bytes are found.
// It is not safe for concurrent use with itself or Read.
func (c *Conn) ReadBytes(ctx context.Context, delim byte) ([]byte, error) {
	// Enable immediate connection cancelation via context by using the context's
	// deadline and also setting a deadline in the past if/when the context is
	// canceled. This pattern courtesy of @acln from #networking on Gophers Slack.
	dl, _ := ctx.Deadline()
	if err := c.conn.SetReadDeadline(dl); err != nil {
		return nil, err
	}

	ch := make(chan rret, 1)
	go func() {
		out, err := c.reader.ReadBytes(delim)
		ch <- rret{out, err}
	}()

	select {
	case <-ctx.Done():
		// Set deadline to unblock pending Write.
		if err := c.conn.SetReadDeadline(aLongTimeAgo); err != nil {
			return nil, err
		}
		// Wait for goroutine to exit, throwing away the error.
		<-ch
		// Reset deadline again.
		if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
			return nil, err
		}
		return nil, ctx.Err()
	case ret := <-ch:
		return ret.val, ret.err
	}
}
