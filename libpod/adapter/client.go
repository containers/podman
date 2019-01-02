// +build remoteclient

package adapter

import (
	"github.com/varlink/go/varlink"
)

// Connect provides a varlink connection
func (r RemoteRuntime) Connect() (*varlink.Connection, error) {
	connection, err := varlink.NewConnection("unix:/run/podman/io.podman")
	if err != nil {
		return nil, err
	}
	return connection, nil
}
