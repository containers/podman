package vmconfigs

import (
	"github.com/containers/podman/v5/pkg/machine/qemu/command"
)

type QEMUConfig struct {
	cmd command.QemuCmd //nolint:unused
}

// Stubs
type AppleHVConfig struct{}
type HyperVConfig struct{}
type WSLConfig struct{}
