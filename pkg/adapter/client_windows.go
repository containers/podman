// +build remoteclient

package adapter

import (
	"github.com/containers/libpod/cmd/podman/remoteclientconfig"
	"github.com/containers/libpod/libpod"
)

func newBridgeConnection(formattedBridge string, remoteConn *remoteclientconfig.RemoteConnection, logLevel string) (*Endpoint, error) {
	// TODO
	// Unix and Windows appear to quote their ssh implementations differently therefore once we figure out what
	// windows ssh is doing here, we can then get the format correct.
	return nil, libpod.ErrNotImplemented
}
