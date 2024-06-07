//go:build !windows && !darwin

package provider

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/qemu"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
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
	resolvedVMType, err := define.ParseVMType(provider, define.QemuVirt)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Using Podman machine with `%s` virtualization provider", resolvedVMType.String())
	switch resolvedVMType {
	case define.QemuVirt:
		return qemu.NewStubber()
	default:
		return nil, fmt.Errorf("unsupported virtualization provider: `%s`", resolvedVMType.String())
	}
}

func GetAll(_ bool) ([]vmconfigs.VMProvider, error) {
	return []vmconfigs.VMProvider{new(qemu.QEMUStubber)}, nil
}

// SupportedProviders returns the providers that are supported on the host operating system
func SupportedProviders() []define.VMType {
	return []define.VMType{define.QemuVirt}
}

// InstalledProviders returns the supported providers that are installed on the host
func InstalledProviders() ([]define.VMType, error) {
	cfg, err := config.Default()
	if err != nil {
		return nil, err
	}
	_, err = cfg.FindHelperBinary(qemu.QemuCommand, true)
	if errors.Is(err, fs.ErrNotExist) {
		return []define.VMType{}, nil
	}
	if err != nil {
		return nil, err
	}

	return []define.VMType{define.QemuVirt}, nil
}

// HasPermsForProvider returns whether the host operating system has the proper permissions to use the given provider
func HasPermsForProvider(provider define.VMType) bool {
	// there are no permissions required for QEMU
	return provider == define.QemuVirt
}
