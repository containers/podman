package mapstructure

import (
	"fmt"
	"reflect"
)

// Error interface is implemented by all errors emitted by mapstructure.
//
// Use [errors.As] to check if an error implements this interface.
type Error interface {
	error

	mapstructure()
}

// DecodeError is a generic error type that holds information about
// a decoding error together with the name of the field that caused the error.
type DecodeError struct {
	name string
	err  error
}

func newDecodeError(name string, err error) *DecodeError {
	return &DecodeError{
		name: name,
		err:  err,
	}
}

func (e *DecodeError) Name() string {
	return e.name
}

func (e *DecodeError) Unwrap() error {
	return e.err
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("'%s' %s", e.name, e.err)
}

func (*DecodeError) mapstructure() {}

// ParseError is an error type that indicates a value could not be parsed
// into the expected type.
type ParseError struct {
	Expected reflect.Value
	Value    any
	Err      error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("cannot parse value as '%s': %s", e.Expected.Type(), e.Err)
}

func (*ParseError) mapstructure() {}

// UnconvertibleTypeError is an error type that indicates a value could not be
// converted to the expected type.
type UnconvertibleTypeError struct {
	Expected reflect.Value
	Value    any
}

func (e *UnconvertibleTypeError) Error() string {
	return fmt.Sprintf(
		"expected type '%s', got unconvertible type '%s'",
		e.Expected.Type(),
		reflect.TypeOf(e.Value),
	)
}

func (*UnconvertibleTypeError) mapstructure() {}
