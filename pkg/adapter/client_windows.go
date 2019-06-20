// +build remoteclient

package adapter

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/remoteclientconfig"
)

func formatDefaultBridge(remoteConn *remoteclientconfig.RemoteConnection, logLevel string) string {
	return fmt.Sprintf(
		`ssh -T %s@%s -- /usr/bin/varlink -A '/usr/bin/podman --log-level=%s varlink $VARLINK_ADDRESS' bridge`,
		remoteConn.Username, remoteConn.Destination, logLevel)
}
