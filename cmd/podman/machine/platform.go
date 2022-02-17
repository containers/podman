//go:build (amd64 && !windows) || (arm64 && !windows)
// +build amd64,!windows arm64,!windows

package machine

import (
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/qemu"
	"github.com/pkg/errors"
)

func getSystemDefaultProvider() machine.Provider {
	return qemu.GetQemuProvider()
}

// Get the sole provider
// Return getSystemDefaultProvider()
func getProvider(filter string) (machine.Provider, error) {
	providers, err := getProviders(filter)
	if len(providers) > 0 {
		return providers[0], err
	}
	return nil, err
}

// Get Provider and VM name by a filter.
// If provided an empty filter, will use default VM names and return the first available
func getProviderByVMName(filter string) (vmName string, provider machine.Provider, err error) {
	var (
		providers []machine.Provider
	)

	providers, err = getProviders("")
	if err != nil {
		return vmName, provider, err
	}

	// If filter is provided will looking for VM by its name
	if filter != "" {
		for _, provider = range providers {
			validVM, err := provider.IsValidVMName(filter)
			if validVM {
				return filter, provider, err
			}
		}
		return vmName, provider, errors.Errorf("vm %s not found", vmName)
	}

	// If filter is empty will looking for the first default VM
	for _, provider = range providers {
		vmName = provider.DefaultVMName()
		_, err = provider.LoadVMByName(vmName)
		if err == nil {
			return vmName, provider, err
		}
	}
	return vmName, provider, errors.Errorf("error, no VMs found")
}
