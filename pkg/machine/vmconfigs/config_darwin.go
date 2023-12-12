package vmconfigs

import (
	"github.com/containers/podman/v4/pkg/machine/applehv/vfkit"
)

type AppleHVConfig struct {
	// The VFKit endpoint where we can interact with the VM
	Vfkit vfkit.VfkitHelper
}

// Stubs
type HyperVConfig struct{}
type WSLConfig struct{}
type QEMUConfig struct{}
