package vmconfigs

import (
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/hyperv/vsock"
	"github.com/containers/podman/v5/pkg/machine/qemu/command"
)

type HyperVConfig struct {
	// ReadyVSock is the pipeline for the guest to alert the host
	// it is running
	ReadyVsock vsock.HVSockRegistryEntry
	// NetworkVSock is for the user networking
	NetworkVSock vsock.HVSockRegistryEntry
}

type WSLConfig struct {
	// Uses usermode networking
	UserModeNetworking bool
}

type QEMUConfig struct {
	// QMPMonitor is the qemu monitor object for sending commands
	QMPMonitor command.Monitor
	// QEMUPidPath is where to write the PID for QEMU when running
	QEMUPidPath *define.VMFile
}

// Stubs
type AppleHVConfig struct{}
type LibKrunConfig struct{}

func getHostUID() int {
	return 1000
}
