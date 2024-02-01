package vmconfigs

import (
	"github.com/containers/podman/v5/pkg/machine/hyperv/vsock"
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

// Stubs
type QEMUConfig struct{}
type AppleHVConfig struct{}

func getHostUID() int {
	return 1000
}
