//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"fmt"

	"github.com/containers/common/pkg/config"
	"github.com/pkg/errors"
)

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
		URI: uri.String(),
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

func RemoveConnection(name string) error {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Engine.ServiceDestinations[name]; ok {
		delete(cfg.Engine.ServiceDestinations, name)
	} else {
		return errors.Errorf("unable to find connection named %q", name)
	}
	return cfg.Write()
}
