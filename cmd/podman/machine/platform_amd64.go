//go:build amd64 && !windows
// +build amd64,!windows

package machine

import (
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/vbox"
	"github.com/pkg/errors"
)

func getVirtualBoxProvider() machine.Provider {
	// override default name if non-system provider
	return vbox.GetVBoxProvider()
}

func getProviders(filter string) ([]machine.Provider, error) {
	switch filter {
	case "":
		return []machine.Provider{getSystemDefaultProvider(), getVirtualBoxProvider()}, nil
	case "system":
		return []machine.Provider{getSystemDefaultProvider()}, nil
	case "vbox":
		return []machine.Provider{getVirtualBoxProvider()}, nil
	default:
		errMsg := `specified unsupported provider type in --type argument: %s. Supported types are: "system", "vbox"`
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
