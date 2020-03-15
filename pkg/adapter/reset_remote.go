// +build remoteclient

package adapter

import (
	"github.com/containers/libpod/cmd/podman/varlink"
)

// Info returns information for the host system and its components
func (r RemoteRuntime) Reset() error {
	return iopodman.Reset().Call(r.Conn)
}
