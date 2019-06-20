// +build remoteclient

package adapter

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/remoteclientconfig"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/varlink/go/varlink"
)

var remoteEndpoint *Endpoint

func (r RemoteRuntime) RemoteEndpoint() (remoteEndpoint *Endpoint, err error) {
	remoteConfigConnections, _ := remoteclientconfig.ReadRemoteConfig(r.config)

	// If the user defines an env variable for podman_varlink_bridge
	// we use that as passed.
	if bridge := os.Getenv("PODMAN_VARLINK_BRIDGE"); bridge != "" {
		logrus.Debug("creating a varlink bridge based on env variable")
		remoteEndpoint, err = newBridgeConnection(bridge, nil, r.cmd.LogLevel)
		// if an environment variable for podman_varlink_address is defined,
		// we used that as passed
	} else if address := os.Getenv("PODMAN_VARLINK_ADDRESS"); address != "" {
		logrus.Debug("creating a varlink address based on env variable: %s", address)
		remoteEndpoint, err = newSocketConnection(address)
		//	if the user provides a remote host, we use it to configure a bridge connection
	} else if len(r.cmd.RemoteHost) > 0 {
		logrus.Debug("creating a varlink bridge based on user input")
		if len(r.cmd.RemoteUserName) < 1 {
			return nil, errors.New("you must provide a username when providing a remote host name")
		}
		rc := remoteclientconfig.RemoteConnection{r.cmd.RemoteHost, r.cmd.RemoteUserName, false}
		remoteEndpoint, err = newBridgeConnection("", &rc, r.cmd.LogLevel)
		//  if the user has a config file with connections in it
	} else if len(remoteConfigConnections.Connections) > 0 {
		logrus.Debug("creating a varlink bridge based configuration file")
		var rc *remoteclientconfig.RemoteConnection
		if len(r.cmd.ConnectionName) > 0 {
			rc, err = remoteConfigConnections.GetRemoteConnection(r.cmd.ConnectionName)
		} else {
			rc, err = remoteConfigConnections.GetDefault()
		}
		if err != nil {
			return nil, err
		}
		remoteEndpoint, err = newBridgeConnection("", rc, r.cmd.LogLevel)
		//	last resort is to make a socket connection with the default varlink address for root user
	} else {
		logrus.Debug("creating a varlink address based default root address")
		remoteEndpoint, err = newSocketConnection(DefaultAddress)
	}
	return
}

// Connect provides a varlink connection
func (r RemoteRuntime) Connect() (*varlink.Connection, error) {
	ep, err := r.RemoteEndpoint()
	if err != nil {
		return nil, err
	}

	switch ep.Type {
	case DirectConnection:
		return varlink.NewConnection(ep.Connection)
	case BridgeConnection:
		return varlink.NewBridge(ep.Connection)
	}
	return nil, errors.New(fmt.Sprintf("Unable to determine type of varlink connection: %s", ep.Connection))
}

// RefreshConnection is used to replace the current r.Conn after things like
// using an upgraded varlink connection
func (r RemoteRuntime) RefreshConnection() error {
	newConn, err := r.Connect()
	if err != nil {
		return err
	}
	r.Conn = newConn
	return nil
}

// newSocketConnection returns an endpoint for a uds based connection
func newSocketConnection(address string) (*Endpoint, error) {
	endpoint := Endpoint{
		Type:       DirectConnection,
		Connection: address,
	}
	return &endpoint, nil
}

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
	endpoint.Connection = formatDefaultBridge(remoteConn, logLevel)
	return &endpoint, nil
}
