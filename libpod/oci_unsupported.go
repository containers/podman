// +build !linux

package libpod

import (
	"os"
	"os/exec"

	"github.com/containers/libpod/libpod/define"
)

func (r *OCIRuntime) moveConmonToCgroup(ctr *Container, cgroupParent string, cmd *exec.Cmd) error {
	return define.ErrOSNotSupported
}

func newPipe() (parent *os.File, child *os.File, err error) {
	return nil, nil, define.ErrNotImplemented
}

func (r *OCIRuntime) createContainer(ctr *Container, cgroupParent string, restoreOptions *ContainerCheckpointOptions) (err error) {
	return define.ErrNotImplemented
}

func (r *OCIRuntime) pathPackage() string {
	return ""
}

func (r *OCIRuntime) conmonPackage() string {
	return ""
}

func (r *OCIRuntime) createOCIContainer(ctr *Container, cgroupParent string, restoreOptions *ContainerCheckpointOptions) (err error) {
	return define.ErrOSNotSupported
}

func (r *OCIRuntime) execStopContainer(ctr *Container, timeout uint) error {
	return define.ErrOSNotSupported
}

func (r *OCIRuntime) stopContainer(ctr *Container, timeout uint) error {
	return define.ErrOSNotSupported
}
