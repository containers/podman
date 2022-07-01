package capnp

import (
	"capnproto.org/go/capnp/v3/exc"
)

var (
	capnperr = exc.Annotator("capnp")
)

// TODO(someday):  progressively remove exported functions and instead
//                 rely on package 'exc'.

// Unimplemented returns an error that formats as the given text and
// will report true when passed to IsUnimplemented.
func Unimplemented(s string) error {
	return exc.New(exc.Unimplemented, "", s)
}

// IsUnimplemented reports whether e indicates that functionality is unimplemented.
func IsUnimplemented(e error) bool {
	return exc.TypeOf(e) == exc.Unimplemented
}

// Disconnected returns an error that formats as the given text and
// will report true when passed to IsDisconnected.
func Disconnected(s string) error {
	return exc.New(exc.Disconnected, "", s)
}

// IsDisconnected reports whether e indicates a failure due to loss of a necessary capability.
func IsDisconnected(e error) bool {
	return exc.TypeOf(e) == exc.Disconnected
}

func errorf(format string, args ...interface{}) error {
	return capnperr.Failedf(format, args...)
}

func annotatef(err error, format string, args ...interface{}) error {
	return capnperr.Annotatef(err, format, args...)
}
