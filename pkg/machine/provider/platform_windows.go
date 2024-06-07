package provider

import (
	"fmt"
	"os"

	"github.com/containers/libhvee/pkg/hypervctl"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/containers/podman/v5/pkg/machine/wsl"
	"github.com/containers/podman/v5/pkg/machine/wsl/wutil"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/hyperv"
	"github.com/sirupsen/logrus"
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
	switch resolvedVMType {
	case define.WSLVirt:
		return new(wsl.WSLStubber), nil
	case define.HyperVVirt:
		if !wsl.HasAdminRights() {
			return nil, fmt.Errorf("hyperv machines require admin authority")
		}
		return new(hyperv.HyperVStubber), nil
	default:
		return nil, fmt.Errorf("unsupported virtualization provider: `%s`", resolvedVMType.String())
	}
}

func GetAll(force bool) ([]vmconfigs.VMProvider, error) {
	providers := []vmconfigs.VMProvider{
		new(wsl.WSLStubber),
	}
	if !wsl.HasAdminRights() && !force {
		logrus.Warn("managing hyperv machines require admin authority.")
	} else {
		providers = append(providers, new(hyperv.HyperVStubber))
	}
	return providers, nil
}

// SupportedProviders returns the providers that are supported on the host operating system
func SupportedProviders() []define.VMType {
	return []define.VMType{define.HyperVVirt, define.WSLVirt}
}

// InstalledProviders returns the supported providers that are installed on the host
func InstalledProviders() ([]define.VMType, error) {
	installed := []define.VMType{}
	if wutil.IsWSLInstalled() {
		installed = append(installed, define.WSLVirt)
	}

	service, err := hypervctl.NewLocalHyperVService()
	if err == nil {
		installed = append(installed, define.HyperVVirt)
	}
	service.Close()

	return installed, nil
}

// HasPermsForProvider returns whether the host operating system has the proper permissions to use the given provider
func HasPermsForProvider(provider define.VMType) bool {
	switch provider {
	case define.QemuVirt:
		fallthrough
	case define.AppleHvVirt:
		return false
	case define.HyperVVirt:
		return wsl.HasAdminRights()
	}

	return true
}
