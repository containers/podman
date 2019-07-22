// +build !linux

package libpod

import (
	"os"
	"os/exec"

	"github.com/containers/libpod/libpod/define"
	"k8s.io/client-go/tools/remotecommand"
)

func (r *OCIRuntime) moveConmonToCgroup(ctr *Container, cgroupParent string, cmd *exec.Cmd) error {
	return define.ErrOSNotSupported
}

func newPipe() (parent *os.File, child *os.File, err error) {
	return nil, nil, define.ErrNotImplemented
}

func (r *OCIRuntime) createContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (err error) {
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

func (r *OCIRuntime) execContainer(c *Container, cmd, capAdd, env []string, tty bool, cwd, user, sessionID string, streams *AttachStreams, preserveFDs int, resize chan remotecommand.TerminalSize, detachKeys string) (int, chan error, error) {
	return -1, nil, define.ErrOSNotSupported
}
