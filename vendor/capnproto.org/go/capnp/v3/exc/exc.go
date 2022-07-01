// Package exc provides an error type for capnp exceptions.
package exc

import (
	"errors"
	"fmt"
)

// Exception is an error that designates a Cap'n Proto exception.
type Exception struct {
	Type   Type
	Prefix string
	Cause  error
}

// New creates a new error that formats as "<prefix>: <msg>".
// The type can be recovered using the TypeOf() function.
func New(typ Type, prefix, msg string) *Exception {
	return &Exception{typ, prefix, errors.New(msg)}
}

func (e Exception) Error() string {
	if e.Prefix == "" {
		return e.Cause.Error()
	}

	return fmt.Sprintf("%s: %v", e.Prefix, e.Cause)
}

func (e Exception) Unwrap() error { return e.Cause }

func (e Exception) GoString() string {
	return fmt.Sprintf("errors.Error{Type: %s, Prefix: %q, Cause: fmt.Errorf(%q)}",
		e.Type.GoString(),
		e.Prefix,
		e.Cause)
}

// Annotate is creates a new error that formats as "<prefix>: <msg>: <e>".
// If e.Prefix == prefix, the prefix will not be duplicated.
// The returned Error.Type == e.Type.
func (e Exception) Annotate(prefix, msg string) *Exception {
	if prefix != e.Prefix {
		return &Exception{e.Type, prefix, fmt.Errorf("%s: %w", msg, e)}
	}

	return &Exception{e.Type, prefix, fmt.Errorf("%s: %w", msg, e.Cause)}
}

// Annotate creates a new error that formats as "<prefix>: <msg>: <err>".
// If err has the same prefix, then the prefix won't be duplicated.
// The returned error's type will match err's type.
func Annotate(prefix, msg string, err error) *Exception {
	if err == nil {
		return nil
	}

	if ce, ok := err.(*Exception); ok {
		return ce.Annotate(prefix, msg)
	}

	return &Exception{
		Type:   Failed,
		Prefix: prefix,
		Cause:  fmt.Errorf("%s: %w", msg, err),
	}
}

type Annotator string

func (f Annotator) New(t Type, err error) *Exception {
	if err == nil {
		return nil
	}

	return &Exception{
		Type:   t,
		Prefix: string(f),
		Cause:  err,
	}
}

func (f Annotator) Failed(err error) *Exception {
	return f.New(Failed, err)
}

func (f Annotator) Failedf(format string, args ...interface{}) *Exception {
	return f.Failed(fmt.Errorf(format, args...))
}

func (f Annotator) Disconnected(err error) *Exception {
	return f.New(Disconnected, err)
}

func (f Annotator) Disconnectedf(format string, args ...interface{}) *Exception {
	return f.Disconnected(fmt.Errorf(format, args...))
}

func (f Annotator) Unimplemented(err error) *Exception {
	return f.New(Unimplemented, err)
}

func (f Annotator) Unimplementedf(format string, args ...interface{}) *Exception {
	return f.Unimplemented(fmt.Errorf(format, args...))
}

func (f Annotator) Annotate(err error, msg string) *Exception {
	return Annotate(string(f), msg, err)
}

func (f Annotator) Annotatef(err error, format string, args ...interface{}) *Exception {
	return f.Annotate(err, fmt.Sprintf(format, args...))
}
