package provider

import (
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/qemu"
)

func getQemuProvider() (machine.VirtProvider, error) {
	return qemu.VirtualizationProvider(), nil
}
