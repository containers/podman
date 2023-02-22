//go:build arm64 && darwin
// +build arm64,darwin

package applehv

import (
	"time"

	"github.com/containers/podman/v4/pkg/machine"
)

var (
	// vmtype refers to qemu (vs libvirt, krun, etc).
	vmtype = "apple"
)

func GetVirtualizationProvider() machine.VirtProvider {
	return &Virtualization{
		artifact:    machine.None,
		compression: machine.Xz,
		format:      machine.Qcow,
	}
}

const (
	// Some of this will need to change when we are closer to having
	// working code.
	VolumeTypeVirtfs     = "virtfs"
	MountType9p          = "9p"
	dockerSock           = "/var/run/docker.sock"
	dockerConnectTimeout = 5 * time.Second
	apiUpTimeout         = 20 * time.Second
)

type apiForwardingState int

const (
	noForwarding apiForwardingState = iota
	claimUnsupported
	notInstalled
	machineLocal
	dockerGlobal
)

}
