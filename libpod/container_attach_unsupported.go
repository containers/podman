//+build !linux

package libpod

import (
	"k8s.io/client-go/tools/remotecommand"
)

func (c *Container) attach(streams *AttachStreams, keys string, resize <-chan remotecommand.TerminalSize, startContainer bool) error {
	return ErrNotImplemented
}
