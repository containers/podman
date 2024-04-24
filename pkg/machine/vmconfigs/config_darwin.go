package vmconfigs

import (
	"os"

	"github.com/containers/podman/v5/pkg/machine/apple/vfkit"
)

type AppleHVConfig struct {
	// The VFKit endpoint where we can interact with the VM
	Vfkit vfkit.Helper
}

type LibKrunConfig struct {
	KRun vfkit.Helper
}

// Stubs
type HyperVConfig struct{}
type WSLConfig struct{}
type QEMUConfig struct{}

func getHostUID() int {
	return os.Getuid()
}
