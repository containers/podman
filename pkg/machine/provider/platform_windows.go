package provider

import (
	"fmt"
	"os"

	"github.com/containers/libhvee/pkg/hypervctl"
	"github.com/containers/podman/v6/pkg/machine/define"
	"github.com/containers/podman/v6/pkg/machine/hyperv"
	"github.com/containers/podman/v6/pkg/machine/vmconfigs"
	"github.com/containers/podman/v6/pkg/machine/windows"
	"github.com/containers/podman/v6/pkg/machine/wsl"
	"github.com/containers/podman/v6/pkg/machine/wsl/wutil"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
)

func Get() (vmconfigs.VMProvider, error) {
	cfg, err := config.Default()
	if err != nil {
		return nil, err
	}
	provider := cfg.Machine.Provider
	if providerOverride, found := os.LookupEnv("CONTAINERS_MACHINE_PROVIDER"); found {
		provider = providerOverride
	}
	resolvedVMType, err := define.ParseVMType(provider, define.WSLVirt)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Using Podman machine with `%s` virtualization provider", resolvedVMType.String())
	return GetByVMType(resolvedVMType)
}

// GetByVMType takes a VMType (presumably from ParseVMType) and returns the correlating
// VMProvider
func GetByVMType(resolvedVMType define.VMType) (vmconfigs.VMProvider, error) {
	switch resolvedVMType {
	case define.WSLVirt:
		return new(wsl.WSLStubber), nil
	case define.HyperVVirt:
		// Check permissions before returning the Hyper-V provider.
		// Working with Hyper-V requires users to be at least members of the Hyper-V admin group.
		// Init and remove actions have custom use cases and they are checked on the stubber.
		if !hyperv.HasHyperVPermissions() {
			return nil, hyperv.ErrHypervUserNotInAdminGroup
		}
		return new(hyperv.HyperVStubber), nil
	default:
	}
	return nil, fmt.Errorf("unsupported virtualization provider: `%s`", resolvedVMType.String())
}

func GetAll() []vmconfigs.VMProvider {
	providers := []vmconfigs.VMProvider{
		new(wsl.WSLStubber),
	}
	if hyperv.HasHyperVPermissions() {
		providers = append(providers, new(hyperv.HyperVStubber))
	}
	return providers
}

// SupportedProviders returns the providers that are supported on the host operating system
func SupportedProviders() []define.VMType {
	return []define.VMType{define.HyperVVirt, define.WSLVirt}
}

func IsInstalled(provider define.VMType) (bool, error) {
	switch provider {
	case define.WSLVirt:
		return wutil.IsWSLInstalled(), nil
	case define.HyperVVirt:
		service, err := hypervctl.NewLocalHyperVService()
		if err == nil {
			return true, nil
		}
		if service != nil {
			defer service.Close()
		}
		return false, nil
	default:
		return false, nil
	}
}

// HasPermsForProvider returns whether the host operating system has the proper permissions to use the given provider
func HasPermsForProvider(provider define.VMType) bool {
	switch provider {
	case define.QemuVirt:
		fallthrough
	case define.AppleHvVirt:
		return false
	case define.HyperVVirt:
		return windows.HasAdminRights()
	}

	return true
}
