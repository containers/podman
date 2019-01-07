// +build remoteclient

package adapter

import (
	"github.com/containers/libpod/libpod"
)

// Images returns information for the host system and its components
func (r RemoteRuntime) Images() ([]libpod.InfoData, error) {
	conn, err := r.Connect()
	if err != nil {
		return nil, err
	}
	_ = conn
	return nil, nil
}
