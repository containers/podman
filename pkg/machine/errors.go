package machine

import (
	"errors"
	"fmt"

	"github.com/containers/podman/v4/pkg/strongunits"
)

var (
	ErrNoSuchVM         = errors.New("VM does not exist")
	ErrWrongState       = errors.New("VM in wrong state to perform action")
	ErrVMAlreadyExists  = errors.New("VM already exists")
	ErrVMAlreadyRunning = errors.New("VM already running or starting")
	ErrMultipleActiveVM = errors.New("only one VM can be active at a time")
	ErrNotImplemented   = errors.New("functionality not implemented")
)

type ErrVMRunningCannotDestroyed struct {
	Name string
}

func (err *ErrVMRunningCannotDestroyed) Error() string {
	return fmt.Sprintf("running vm %q cannot be destroyed", err.Name)
}

type ErrVMDoesNotExist struct {
	Name string
}

func (err *ErrVMDoesNotExist) Error() string {
	// the current error in qemu is not quoted
	return fmt.Sprintf("%s: VM does not exist", err.Name)
}

type ErrNewDiskSizeTooSmall struct {
	OldSize, NewSize strongunits.GiB
}

func (err *ErrNewDiskSizeTooSmall) Error() string {
	return fmt.Sprintf("invalid disk size %d: new disk must be larger than %dGB", err.OldSize, err.NewSize)
}
