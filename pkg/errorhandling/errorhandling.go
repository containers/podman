package errorhandling

import (
	"os"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// JoinErrors converts the error slice into a single human-readable error.
func JoinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	// `multierror` appends new lines which we need to remove to prevent
	// blank lines when printing the error.
	var multiE *multierror.Error
	multiE = multierror.Append(multiE, errs...)

	finalErr := multiE.ErrorOrNil()
	if finalErr == nil {
		return finalErr
	}
	if len(multiE.WrappedErrors()) == 1 && logrus.IsLevelEnabled(logrus.TraceLevel) {
		return multiE.WrappedErrors()[0]
	}
	return errors.New(strings.TrimSpace(finalErr.Error()))
}

// ErrorsToString converts the slice of errors into a slice of corresponding
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

// SyncQuiet syncs a file and logs any error. Should only be used within
// a defer.
func SyncQuiet(f *os.File) {
	if err := f.Sync(); err != nil {
		logrus.Errorf("unable to sync file %s: %q", f.Name(), err)
	}
}

// CloseQuiet closes a file and logs any error. Should only be used within
// a defer.
func CloseQuiet(f *os.File) {
	if err := f.Close(); err != nil {
		logrus.Errorf("unable to close file %s: %q", f.Name(), err)
	}
}

// Contains checks if err's message contains sub's message. Contains should be
// used iff either err or sub has lost type information (e.g., due to
// marshaling).  For typed errors, please use `errors.Contains(...)` or `Is()`
// in recent version of Go.
func Contains(err error, sub error) bool {
	return strings.Contains(err.Error(), sub.Error())
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
