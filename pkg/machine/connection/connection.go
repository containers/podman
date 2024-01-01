//go:build amd64 || arm64

package connection

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"

	"github.com/containers/common/pkg/config"
	"github.com/sirupsen/logrus"
)

const LocalhostIP = "127.0.0.1"

func AddConnection(uri fmt.Stringer, name, identity string, isDefault bool) error {
	if len(identity) < 1 {
		return errors.New("identity must be defined")
	}

	return config.EditConnectionConfig(func(cfg *config.ConnectionsFile) error {
		if _, ok := cfg.Connection.Connections[name]; ok {
			return errors.New("cannot overwrite connection")
		}

		dst := config.Destination{
			URI:       uri.String(),
			IsMachine: true,
			Identity:  identity,
		}

		if isDefault {
			cfg.Connection.Default = name
		}

		if cfg.Connection.Connections == nil {
			cfg.Connection.Connections = map[string]config.Destination{
				name: dst,
			}
			cfg.Connection.Default = name
		} else {
			cfg.Connection.Connections[name] = dst
		}

		return nil
	})
}

func ChangeConnectionURI(name string, uri fmt.Stringer) error {
	return config.EditConnectionConfig(func(cfg *config.ConnectionsFile) error {
		dst, ok := cfg.Connection.Connections[name]
		if !ok {
			return errors.New("connection not found")
		}
		dst.URI = uri.String()
		cfg.Connection.Connections[name] = dst

		return nil
	})
}

// UpdateConnectionIfDefault updates the default connection to the rootful/rootless when depending
// on the bool but only if other rootful/less connection was already the default.
// Returns true if it modified the default
func UpdateConnectionIfDefault(rootful bool, name, rootfulName string) error {
	return config.EditConnectionConfig(func(cfg *config.ConnectionsFile) error {
		if name == cfg.Connection.Default && rootful {
			cfg.Connection.Default = rootfulName
		} else if rootfulName == cfg.Connection.Default && !rootful {
			cfg.Connection.Default = name
		}
		return nil
	})
}

func RemoveConnections(names ...string) error {
	return config.EditConnectionConfig(func(cfg *config.ConnectionsFile) error {
		for _, name := range names {
			if _, ok := cfg.Connection.Connections[name]; ok {
				delete(cfg.Connection.Connections, name)
			} else {
				return fmt.Errorf("unable to find connection named %q", name)
			}

			if cfg.Connection.Default == name {
				cfg.Connection.Default = ""
			}
		}
		for service := range cfg.Connection.Connections {
			cfg.Connection.Default = service
			break
		}
		return nil
	})
}

// removeFilesAndConnections removes any files and connections with the given names
func RemoveFilesAndConnections(files []string, names ...string) {
	for _, f := range files {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Error(err)
		}
	}
	if err := RemoveConnections(names...); err != nil {
		logrus.Error(err)
	}
}

type RemoteConnectionType string

var SSHRemoteConnection RemoteConnectionType = "ssh"

// MakeSSHURL
func (rc RemoteConnectionType) MakeSSHURL(host, path, port, userName string) url.URL {
	// TODO Should this function have input verification?
	userInfo := url.User(userName)
	uri := url.URL{
		Scheme:     "ssh",
		Opaque:     "",
		User:       userInfo,
		Host:       host,
		Path:       path,
		RawPath:    "",
		ForceQuery: false,
		RawQuery:   "",
		Fragment:   "",
	}
	if len(port) > 0 {
		uri.Host = net.JoinHostPort(uri.Hostname(), port)
	}
	return uri
}
