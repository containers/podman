// +build linux darwin
// +build remoteclient

package adapter

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/remoteclientconfig"
	"github.com/pkg/errors"
)

// newBridgeConnection creates a bridge type endpoint with username, destination, and log-level
func newBridgeConnection(formattedBridge string, remoteConn *remoteclientconfig.RemoteConnection, logLevel string) (*Endpoint, error) {
	endpoint := Endpoint{
		Type: BridgeConnection,
	}

	if len(formattedBridge) < 1 && remoteConn == nil {
		return nil, errors.New("bridge connections must either be created by string or remoteconnection")
	}
	if len(formattedBridge) > 0 {
		endpoint.Connection = formattedBridge
		return &endpoint, nil
	}
	endpoint.Connection = fmt.Sprintf(
		`ssh -T %s@%s -- /usr/bin/varlink -A \'/usr/bin/podman --log-level=%s varlink \\\$VARLINK_ADDRESS\' bridge`,
		remoteConn.Username, remoteConn.Destination, logLevel)
	return &endpoint, nil
}
