package define

import (
	"errors"
	"fmt"

	"github.com/containers/common/pkg/strongunits"
)

var (
	ErrWrongState      = errors.New("VM in wrong state to perform action")
	ErrVMAlreadyExists = errors.New("VM already exists")
	ErrNotImplemented  = errors.New("functionality not implemented")
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

type ErrIncompatibleMachineConfig struct {
	Name string
	Path string
}

func (err *ErrIncompatibleMachineConfig) Error() string {
	return fmt.Sprintf("incompatible machine config %q (%s) for this version of Podman", err.Path, err.Name)
}

type ErrMultipleActiveVM struct {
	Name     string
	Provider string
}

func (err *ErrMultipleActiveVM) Error() string {
	msg := ""
	if err.Provider != "" {
		msg = " on the " + err.Provider + " provider"
	}
	return fmt.Sprintf("%s already starting or running%s: only one VM can be active at a time", err.Name, msg)
}
