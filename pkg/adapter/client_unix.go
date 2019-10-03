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
	options := ""
	if remoteConn.IdentityFile != "" {
		options += " -i " + remoteConn.IdentityFile
	}
	if remoteConn.IgnoreHosts {
		options += " -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
	}
	return fmt.Sprintf(
		`ssh -p %d -T%s %s@%s -- varlink -A \'podman --log-level=%s varlink \\\$VARLINK_ADDRESS\' bridge`,
		port, options, remoteConn.Username, remoteConn.Destination, logLevel)
}
