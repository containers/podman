// +build remoteclient

package adapter

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/varlink/go/varlink"
)

type VarlinkConnectionInfo struct {
	RemoteUserName string
	RemoteHost     string
	VarlinkAddress string
}

// Connect provides a varlink connection
func (r RemoteRuntime) Connect() (*varlink.Connection, error) {
	var (
		err        error
		connection *varlink.Connection
	)

	logLevel := r.cmd.LogLevel

	// I'm leaving this here for now as a document of the birdge format.  It can be removed later once the bridge
	// function is more flushed out.
	//bridge := `ssh -T root@192.168.122.1 "/usr/bin/varlink -A '/usr/bin/podman varlink \$VARLINK_ADDRESS' bridge"`
	if len(r.cmd.RemoteHost) > 0 {
		// The user has provided a remote host endpoint
		if len(r.cmd.RemoteUserName) < 1 {
			return nil, errors.New("you must provide a username when providing a remote host name")
		}
		bridge := fmt.Sprintf(`ssh -T %s@%s /usr/bin/varlink -A \'/usr/bin/podman --log-level=%s varlink \\\$VARLINK_ADDRESS\' bridge`, r.cmd.RemoteUserName, r.cmd.RemoteHost, logLevel)
		connection, err = varlink.NewBridge(bridge)
	} else if bridge := os.Getenv("PODMAN_VARLINK_BRIDGE"); bridge != "" {
		connection, err = varlink.NewBridge(bridge)
	} else {
		address := os.Getenv("PODMAN_VARLINK_ADDRESS")
		if address == "" {
			address = DefaultAddress
		}
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
