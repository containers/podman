// +build linux darwin
// +build remoteclient

package adapter

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/remoteclientconfig"
)

func formatDefaultBridge(remoteConn *remoteclientconfig.RemoteConnection, logLevel string) string {
	port := remoteConn.Port
	if port == 0 {
		port = 22
	}
	return fmt.Sprintf(
		`ssh -p %d -T %s@%s -- /usr/bin/varlink -A \'/usr/bin/podman --log-level=%s varlink \\\$VARLINK_ADDRESS\' bridge`,
		port, remoteConn.Username, remoteConn.Destination, logLevel)
}
