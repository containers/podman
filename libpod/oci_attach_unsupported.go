//+build !linux

package libpod

import (
	"os"

	"github.com/containers/podman/v2/libpod/define"
	"k8s.io/client-go/tools/remotecommand"
)

func (c *Container) attach(streams *define.AttachStreams, keys string, resize <-chan remotecommand.TerminalSize, startContainer bool, started chan bool, attachRdy chan<- bool) error {
	return define.ErrNotImplemented
}

func (c *Container) attachToExec(streams *define.AttachStreams, keys string, resize <-chan remotecommand.TerminalSize, sessionID string, startFd *os.File, attachFd *os.File) error {
	return define.ErrNotImplemented
}
