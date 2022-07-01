package rpc

import (
	"errors"

	"capnproto.org/go/capnp/v3/exc"
)

var (
	rpcerr = exc.Annotator("rpc")

	// Base errors
	ErrConnClosed        = errors.New("connection closed")
	ErrNotACapability    = errors.New("not a capability")
	ErrCapTablePopulated = errors.New("capability table already populated")

	// RPC exceptions
	ExcClosed = rpcerr.Disconnected(ErrConnClosed)
)

type errReporter struct {
	ErrorReporter
}

func (er errReporter) ReportError(err error) {
	if er.ErrorReporter != nil && err != nil {
		er.ErrorReporter.ReportError(err)
	}
}
