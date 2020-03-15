package remoteclientconfig

import "errors"

var (
	// ErrNoDefaultConnection no default connection is defined in the podman-remote.conf file
	ErrNoDefaultConnection = errors.New("no default connection is defined")
	// ErrNoDefinedConnections no connections are defined in the podman-remote.conf file
	ErrNoDefinedConnections = errors.New("no remote connections have been defined")
	// ErrConnectionNotFound unable to lookup connection by name
	ErrConnectionNotFound = errors.New("remote connection not found by name")
	// ErrNoConfigationFile no config file found
	ErrNoConfigationFile = errors.New("no configuration file found")
)
