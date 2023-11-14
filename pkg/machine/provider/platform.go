//go:build !windows && !darwin

package provider

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/qemu"
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
	resolvedVMType, err := define.ParseVMType(provider, define.QemuVirt)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Using Podman machine with `%s` virtualization provider", resolvedVMType.String())
	switch resolvedVMType {
	case define.QemuVirt:
		return qemu.VirtualizationProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported virtualization provider: `%s`", resolvedVMType.String())
	}
}
