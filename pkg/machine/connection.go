//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"errors"
	"fmt"
	"os"

	"github.com/containers/common/pkg/config"
	"github.com/sirupsen/logrus"
)

const LocalhostIP = "127.0.0.1"

func AddConnection(uri fmt.Stringer, name, identity string, isDefault bool) error {
	if len(identity) < 1 {
		return errors.New("identity must be defined")
	}
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Engine.ServiceDestinations[name]; ok {
		return errors.New("cannot overwrite connection")
	}
	if isDefault {
		cfg.Engine.ActiveService = name
	}
	dst := config.Destination{
		URI:       uri.String(),
		IsMachine: true,
	}
	dst.Identity = identity
	if cfg.Engine.ServiceDestinations == nil {
		cfg.Engine.ServiceDestinations = map[string]config.Destination{
			name: dst,
		}
		cfg.Engine.ActiveService = name
	} else {
		cfg.Engine.ServiceDestinations[name] = dst
	}
	return cfg.Write()
}

func AnyConnectionDefault(name ...string) (bool, error) {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return false, err
	}
	for _, n := range name {
		if n == cfg.Engine.ActiveService {
			return true, nil
		}
	}

	return false, nil
}

func ChangeConnectionURI(name string, uri fmt.Stringer) error {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}
	dst, ok := cfg.Engine.ServiceDestinations[name]
	if !ok {
		return errors.New("connection not found")
	}
	dst.URI = uri.String()
	cfg.Engine.ServiceDestinations[name] = dst

	return cfg.Write()
}

func ChangeDefault(name string) error {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}

	cfg.Engine.ActiveService = name

	return cfg.Write()
}

func RemoveConnections(names ...string) error {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}
	for _, name := range names {
		if _, ok := cfg.Engine.ServiceDestinations[name]; ok {
			delete(cfg.Engine.ServiceDestinations, name)
		} else {
			return fmt.Errorf("unable to find connection named %q", name)
		}

		if cfg.Engine.ActiveService == name {
			cfg.Engine.ActiveService = ""
			for service := range cfg.Engine.ServiceDestinations {
				cfg.Engine.ActiveService = service
				break
			}
		}
	}
	return cfg.Write()
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
