//+build !linux

package libpod

import (
	"sync"

	"k8s.io/client-go/tools/remotecommand"
)

func (c *Container) attach(streams *AttachStreams, keys string, resize <-chan remotecommand.TerminalSize, startContainer bool, wg *sync.WaitGroup) error {
	return ErrNotImplemented
}
