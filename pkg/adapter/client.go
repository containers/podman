// +build remoteclient

package adapter

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/varlink/go/varlink"
)

// DefaultAddress is the default address of the varlink socket
const DefaultAddress = "unix:/run/podman/io.podman"

// Connect provides a varlink connection
func (r RemoteRuntime) Connect() (*varlink.Connection, error) {
	var err error
	var connection *varlink.Connection
	if bridge := os.Getenv("PODMAN_VARLINK_BRIDGE"); bridge != "" {
		logrus.Infof("Connecting with varlink bridge")
		logrus.Debugf("%s", bridge)
		connection, err = varlink.NewBridge(bridge)
	} else {
		address := os.Getenv("PODMAN_VARLINK_ADDRESS")
		if address == "" {
			address = DefaultAddress
		}
		logrus.Infof("Connecting with varlink address")
		logrus.Debugf("%s", address)
		connection, err = varlink.NewConnection(address)
	}
	if err != nil {
		return nil, err
	}
	return connection, nil
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
