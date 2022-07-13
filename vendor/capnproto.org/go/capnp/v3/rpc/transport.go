package rpc

// Re-export things from the transport package

import (
	"io"

	"capnproto.org/go/capnp/v3/rpc/transport"
)

type Codec = transport.Codec
type Transport = transport.Transport

// NewStreamTransport is an alias for as transport.NewStream
func NewStreamTransport(rwc io.ReadWriteCloser) Transport {
	return transport.NewStream(rwc)
}

// NewPackedStreamTransport is an alias for as transport.NewPackedStream
func NewPackedStreamTransport(rwc io.ReadWriteCloser) Transport {
	return transport.NewPackedStream(rwc)
}

// NewTransport is an alias for as transport.New
func NewTransport(codec Codec) Transport {
	return transport.New(codec)
}
