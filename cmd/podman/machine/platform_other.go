//go:build arm64 || windows
// +build arm64 windows

package machine

import (
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/pkg/errors"
)

func getProviders(filter string) ([]machine.Provider, error) {
	switch filter {
	case "", "system":
		return []machine.Provider{getSystemDefaultProvider()}, nil
	default:
		errMsg := `specified unsupported provider type in --type argument: %s. Supported type is: "system"`
		return nil, errors.Errorf(errMsg, providerType)
	}
}

func getProvider(filter string) (machine.Provider, error) {
	providers, err := getProviders(filter)
	if len(providers) > 0 {
		return providers[0], err
	}
	return nil, err
}
