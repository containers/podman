//+build !linux

package libpod

import (
	"os"

	"github.com/containers/podman/v3/libpod/define"
)

func (c *Container) attach(streams *define.AttachStreams, keys string, resize <-chan define.TerminalSize, startContainer bool, started chan bool, attachRdy chan<- bool) error {
	return define.ErrNotImplemented
}

func (c *Container) attachToExec(streams *define.AttachStreams, keys string, resize <-chan define.TerminalSize, sessionID string, startFd *os.File, attachFd *os.File) error {
	return define.ErrNotImplemented
}
