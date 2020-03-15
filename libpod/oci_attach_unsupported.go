//+build !linux

package libpod

import (
	"os"

	"github.com/containers/libpod/libpod/define"
	"k8s.io/client-go/tools/remotecommand"
)

func (c *Container) attach(streams *AttachStreams, keys string, resize <-chan remotecommand.TerminalSize, startContainer bool, started chan bool) error {
	return define.ErrNotImplemented
}

func (c *Container) attachToExec(streams *AttachStreams, keys string, resize <-chan remotecommand.TerminalSize, sessionID string, startFd *os.File, attachFd *os.File) error {
	return define.ErrNotImplemented
}
