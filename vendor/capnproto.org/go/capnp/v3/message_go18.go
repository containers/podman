//go:build go1.8
// +build go1.8

package capnp

import "net"

func (e *Encoder) write(bufs [][]byte) error {
	_, err := (*net.Buffers)(&bufs).WriteTo(e.w)
	return err
}
