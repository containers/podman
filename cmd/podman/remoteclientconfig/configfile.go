package remoteclientconfig

import (
	"io"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
)

// ReadRemoteConfig takes an io.Reader representing the remote configuration
// file and returns a remoteconfig
func ReadRemoteConfig(reader io.Reader) (*RemoteConfig, error) {
	var remoteConfig RemoteConfig
	// the configuration file does not exist
	if reader == nil {
		return &remoteConfig, ErrNoConfigationFile
	}
	_, err := toml.DecodeReader(reader, &remoteConfig)
	if err != nil {
		return nil, err
	}
	// We need to validate each remote connection has fields filled out
	for name, conn := range remoteConfig.Connections {
		if len(conn.Destination) < 1 {
			return nil, errors.Errorf("connection %q has no destination defined", name)
		}
	}
	return &remoteConfig, err
}

// GetDefault returns the default RemoteConnection. If there is only one
// connection, we assume it is the default as well
func (r *RemoteConfig) GetDefault() (*RemoteConnection, error) {
	if len(r.Connections) == 0 {
		return nil, ErrNoDefinedConnections
	}
	for _, v := range r.Connections {
		v := v
		if len(r.Connections) == 1 {
			// if there is only one defined connection, we assume it is
			// the default whether tagged as such or not
			return &v, nil
		}
		if v.IsDefault {
			return &v, nil
		}
	}
	return nil, ErrNoDefaultConnection
}

// GetRemoteConnection "looks up" a remote connection by name and returns it in the
// form of a RemoteConnection
func (r *RemoteConfig) GetRemoteConnection(name string) (*RemoteConnection, error) {
	if len(r.Connections) == 0 {
		return nil, ErrNoDefinedConnections
	}
	for k, v := range r.Connections {
		v := v
		if k == name {
			return &v, nil
		}
	}
	return nil, errors.Wrap(ErrConnectionNotFound, name)
}
