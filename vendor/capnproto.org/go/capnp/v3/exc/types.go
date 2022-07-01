package exc

import (
	"errors"
	"strconv"
)

// Type indicates the type of error, mirroring those in rpc.capnp.
type Type int

// Error types.
const (
	Failed        Type = 0
	Overloaded    Type = 1
	Disconnected  Type = 2
	Unimplemented Type = 3
)

// String returns the lowercased Go constant name, or a string in the
// form "type(X)" where X is the value of typ for any unrecognized type.
func (typ Type) String() string {
	switch typ {
	case Failed:
		return "failed"
	case Overloaded:
		return "overloaded"
	case Disconnected:
		return "disconnected"
	case Unimplemented:
		return "unimplemented"
	default:
		var buf [26]byte
		s := append(buf[:0], "type("...)
		s = strconv.AppendInt(s, int64(typ), 10)
		s = append(s, ')')
		return string(s)
	}
}

// GoString returns the Go constant name, or a string in the form
// "Type(X)" where X is the value of typ for any unrecognized type.
func (typ Type) GoString() string {
	switch typ {
	case Failed:
		return "Failed"
	case Overloaded:
		return "Overloaded"
	case Disconnected:
		return "Disconnected"
	case Unimplemented:
		return "Unimplemented"
	default:
		var buf [26]byte
		s := append(buf[:0], "Type("...)
		s = strconv.AppendInt(s, int64(typ), 10)
		s = append(s, ')')
		return string(s)
	}
}

// TypeOf returns err's type if err was created by this package or
// Failed if it was not.
func TypeOf(err error) Type {
	ce, ok := err.(*Exception)
	if !ok {
		return Failed
	}
	return ce.Type
}

// IsType reports whether any error in err's is an Exception
// whose type matches 't'.
//
// The chain consists of err itself followed by the sequence of
// errors obtained by repeatedly calling Unwrap.
func IsType(err error, t Type) bool {
	var exc = &Exception{}
	if errors.As(err, &exc) {
		return exc.Type == t
	}

	return false
}
