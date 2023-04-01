package provider

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/hyperv"
	"github.com/containers/podman/v4/pkg/machine/wsl"
	"github.com/sirupsen/logrus"
)

func Get() (machine.VirtProvider, error) {
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
		return wsl.VirtualizationProvider(), nil
	case define.HyperVVirt:
		return hyperv.VirtualizationProvider(), nil
	case define.QemuVirt:
		return getQemuProvider()
	default:
		return nil, fmt.Errorf("unsupported virtualization provider: `%s`", resolvedVMType.String())
	}
}
