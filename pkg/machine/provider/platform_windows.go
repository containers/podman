package provider

import (
	"fmt"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"os"

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
	// TODO re-enable this with WSL
	//case define.WSLVirt:
	//	return wsl.VirtualizationProvider(), nil
	case define.HyperVVirt:
		return new(hyperv.HyperVStubber), nil
	default:
		return nil, fmt.Errorf("unsupported virtualization provider: `%s`", resolvedVMType.String())
	}
}
