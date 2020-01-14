// +build go1.13

package errors

import (
	stderrors "errors"
)

// Is reports whether any error in err's chain matches target.
//
// The chain consists of err itself followed by the sequence of errors obtained by
// repeatedly calling Unwrap.
//
// An error is considered to match a target if it is equal to that target or if
// it implements a method Is(error) bool such that Is(target) returns true.
func Is(err, target error) bool { return stderrors.Is(err, target) }

// As finds the first error in err's chain that matches target, and if so, sets
// target to that error value and returns true.
//
// The chain consists of err itself followed by the sequence of errors obtained by
// repeatedly calling Unwrap.
//
// An error matches target if the error's concrete value is assignable to the value
// pointed to by target, or if the error has a method As(interface{}) bool such that
// As(target) returns true. In the latter case, the As method is responsible for
// setting target.
//
// As will panic if target is not a non-nil pointer to either a type that implements
// error, or to any interface type. As returns false if err is nil.
func As(err error, target interface{}) bool { return stderrors.As(err, target) }

// Unwrap returns the result of calling the Unwrap method on err, if err's
// type contains an Unwrap method returning error.
// Otherwise, Unwrap returns nil.
func Unwrap(err error) error {
	return stderrors.Unwrap(err)
}

// Cause recursively unwraps an error chain and returns the underlying cause of
// the error, if possible. There are two ways that an error value may provide a
// cause. First, the error may implement the following interface:
//
//     type causer interface {
//            Cause() error
//     }
//
// Second, the error may return a non-nil value when passed as an argument to
// the Unwrap function. This makes Cause forwards-compatible with Go 1.13 error
// chains.
//
// If an error value satisfies both methods of unwrapping, Cause will use the
// causer interface.
//
// If the error is nil, nil will be returned without further investigation.
func Cause(err error) error {
	type causer interface {
		Cause() error
	}

	for err != nil {
		if cause, ok := err.(causer); ok {
			err = cause.Cause()
		} else if unwrapped := Unwrap(err); unwrapped != nil {
			err = unwrapped
		} else {
			break
		}
	}
	return err
}
