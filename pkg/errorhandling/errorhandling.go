package errorhandling

import (
	"os"
	"strings"
	"unsafe"

	"github.com/hashicorp/go-multierror"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func init() {
	jsoniter.RegisterTypeEncoderFunc("[]error", MarshalErrorSliceJSON, MarshalErrorSliceJSONIsEmpty)
	jsoniter.RegisterTypeEncoderFunc("error", MarshalErrorJSON, MarshalErrorJSONIsEmpty)
}

// JoinErrors converts the error slice into a single human-readable error.
func JoinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	// If there's just one error, return it.  This prevents the "%d errors
	// occurred:" header plus list from the multierror package.
	if len(errs) == 1 {
		return errs[0]
	}

	// `multierror` appends new lines which we need to remove to prevent
	// blank lines when printing the error.
	var multiE *multierror.Error
	multiE = multierror.Append(multiE, errs...)

	finalErr := multiE.ErrorOrNil()
	if finalErr == nil {
		return nil
	}
	return errors.New(strings.TrimSpace(finalErr.Error()))
}

// ErrorsToStrings converts the slice of errors into a slice of corresponding
// error messages.
func ErrorsToStrings(errs []error) []string {
	if len(errs) == 0 {
		return nil
	}
	strErrs := make([]string, len(errs))
	for i := range errs {
		strErrs[i] = errs[i].Error()
	}
	return strErrs
}

// StringsToErrors converts a slice of error messages into a slice of
// corresponding errors.
func StringsToErrors(strErrs []string) []error {
	if len(strErrs) == 0 {
		return nil
	}
	errs := make([]error, len(strErrs))
	for i := range strErrs {
		errs[i] = errors.New(strErrs[i])
	}
	return errs
}

// SyncQuiet syncs a file and logs any error. Should only be used within a defer.
func SyncQuiet(f *os.File) {
	if err := f.Sync(); err != nil {
		logrus.Errorf("Unable to sync file %s: %q", f.Name(), err)
	}
}

// CloseQuiet closes a file and logs any error. Should only be used within a defer.
func CloseQuiet(f *os.File) {
	if err := f.Close(); err != nil {
		logrus.Errorf("Unable to close file %s: %q", f.Name(), err)
	}
}

// Contains checks if err's message contains sub's message. Contains should be
// used iff either err or sub has lost type information (e.g., due to
// marshaling).  For typed errors, please use `errors.Contains(...)` or `Is()`
// in recent version of Go.
func Contains(err error, sub error) bool {
	return strings.Contains(err.Error(), sub.Error())
}

// PodConflictErrorModel is used in remote connections with podman
type PodConflictErrorModel struct {
	Errs []string
	Id   string // nolint
}

// ErrorModel is used in remote connections with podman
type ErrorModel struct {
	// API root cause formatted for automated parsing
	// example: API root cause
	Because string `json:"cause"`
	// human error message, formatted for a human to read
	// example: human error message
	Message string `json:"message"`
	// http response code
	ResponseCode int `json:"response"`
}

func (e ErrorModel) Error() string {
	return e.Message
}

func (e ErrorModel) Cause() error {
	return errors.New(e.Because)
}

func (e ErrorModel) Code() int {
	return e.ResponseCode
}

func (e PodConflictErrorModel) Error() string {
	return strings.Join(e.Errs, ",")
}

func (e PodConflictErrorModel) Code() int {
	return 409
}

// MarshalErrorJSON writes error to stream as string
func MarshalErrorJSON(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	p := *((*error)(ptr))
	if p == nil {
		stream.WriteNil()
	} else {
		stream.WriteString(p.Error())
	}
}

// MarshalErrorSliceJSON writes []error to stream as []string JSON blob
func MarshalErrorSliceJSON(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	a := *((*[]error)(ptr))
	switch {
	case a == nil:
		stream.WriteNil()
	case len(a) == 0:
		stream.WriteEmptyArray()
	default:
		stream.WriteArrayStart()
		for i, e := range a {
			if i > 0 {
				stream.WriteMore()
			}
			if e == nil {
				stream.WriteNil()
			} else {
				stream.WriteString(e.Error())
			}
		}
		stream.WriteArrayEnd()
	}
}

func MarshalErrorJSONIsEmpty(ptr unsafe.Pointer) bool {
	return *((*error)(ptr)) == nil
}

func MarshalErrorSliceJSONIsEmpty(ptr unsafe.Pointer) bool {
	return len(*((*[]error)(ptr))) == 0
}

// UnmarshalErrorJSON decodes string as error
// Note: a _NEW_ error is created with matching text, therefore
//     MarshalErrorJSON(&os.ErrNotExist, stream)
//     UnmarshalErrorJSON(&err, stream)
//     err != os.ErrNotExist && err.Error() == os.ErrNotExist.Error()
func UnmarshalErrorJSON(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	switch iter.WhatIsNext() {
	case jsoniter.StringValue:
		*(*error)(ptr) = errors.New(iter.ReadString())
	case jsoniter.NilValue:
		iter.ReadNil()
		*(*error)(ptr) = nil
	default:
		iter.ReportError("podman decode error", "unsupported type in payload")
	}
}

// UnmarshalErrorSliceJSON decodes a slice of strings as a slice of errors
func UnmarshalErrorSliceJSON(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	slice := make([]error, 0)
	for iter.ReadArray() {
		switch iter.WhatIsNext() {
		case jsoniter.StringValue:
			slice = append(slice, errors.New(iter.ReadString()))
		case jsoniter.NilValue:
			iter.ReadNil()
			slice = append(slice, nil)
		default:
			iter.ReportError("podman decode error", "unsupported type in payload")
			return
		}
	}
	*((*[]error)(ptr)) = slice
}
