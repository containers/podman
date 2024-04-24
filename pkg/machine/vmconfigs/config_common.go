//go:build linux || freebsd

package vmconfigs

import (
	"os"

	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/qemu/command"
)

type QEMUConfig struct {
	// QMPMonitor is the qemu monitor object for sending commands
	QMPMonitor command.Monitor
	// QEMUPidPath is where to write the PID for QEMU when running
	QEMUPidPath *define.VMFile
}

// Stubs
type AppleHVConfig struct{}
type HyperVConfig struct{}
type LibKrunConfig struct{}
type WSLConfig struct{}

func getHostUID() int {
	return os.Getuid()
}
