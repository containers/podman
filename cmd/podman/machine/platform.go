//go:build (amd64 && !windows) || (arm64 && !windows)
// +build amd64,!windows arm64,!windows

package machine

import (
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/qemu"
)

func GetSystemDefaultProvider() machine.Provider {
	return qemu.GetVirtualizationProvider()
}
