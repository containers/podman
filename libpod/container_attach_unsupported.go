//+build !linux

package libpod

import (
	"sync"

	"github.com/containers/libpod/libpod/define"
	"k8s.io/client-go/tools/remotecommand"
)

func (c *Container) attach(streams *AttachStreams, keys string, resize <-chan remotecommand.TerminalSize, startContainer bool, wg *sync.WaitGroup) error {
	return define.ErrNotImplemented
}
