//go:build windows

package hypervctl

import (
	"errors"
	"strings"
)

// VM Creation errors
var (
	ErrMachineAlreadyExists  = errors.New("machine already exists")
	ErrMachineStateInvalid   = errors.New("machine in invalid state for action")
	ErrMachineNotRunning     = errors.New("machine not running")
	ErrMachineAlreadyRunning = errors.New("machine already running")
)

func NewPSError(stderr string) error {
	strArr := strings.Split(stderr, "\n")
	// take the first line as it contains the error message
	if len(strArr) > 0 {
		return errors.New(strArr[0])
	}
	return errors.New(stderr)
}
