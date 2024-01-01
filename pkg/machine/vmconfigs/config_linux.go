package vmconfigs

import (
	"os"

	"github.com/containers/podman/v4/pkg/machine/qemu/command"
)

type QEMUConfig struct {
	Command command.QemuCmd
	// QMPMonitor is the qemu monitor object for sending commands
	QMPMonitor command.Monitor
}

// Stubs
type AppleHVConfig struct{}
type HyperVConfig struct{}
type WSLConfig struct{}

func getHostUID() int {
	return os.Getuid()
}
