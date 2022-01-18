// +build amd64,!windows arm64,!windows

package machine

import (
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/qemu"
)

func getSystemDefaultProvider() machine.Provider {
	return qemu.GetQemuProvider()
}
