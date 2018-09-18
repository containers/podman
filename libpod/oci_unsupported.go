// +build !linux

package libpod

import (
	"os"
	"os/exec"
)

func (r *OCIRuntime) moveConmonToCgroup(ctr *Container, cgroupParent string, cmd *exec.Cmd) error {
	return ErrOSNotSupported
}

func newPipe() (parent *os.File, child *os.File, err error) {
	return nil, nil, ErrNotImplemented
}

func (r *OCIRuntime) createContainer(ctr *Container, cgroupParent string, restoreContainer bool) (err error) {
	return ErrNotImplemented
}

func (r *OCIRuntime) pathPackage() string {
	return ""
}

func (r *OCIRuntime) conmonPackage() string {
	return ""
}
