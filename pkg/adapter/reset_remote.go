// +build remoteclient

package adapter

import (
	iopodman "github.com/containers/libpod/pkg/varlink"
)

// Info returns information for the host system and its components
func (r RemoteRuntime) Reset() error {
	return iopodman.Reset().Call(r.Conn)
}
