package vmconfigs

import (
	"github.com/containers/podman/v4/pkg/machine/hyperv/vsock"
)

type HyperVConfig struct {
	// NetworkVSock is for the user networking
	NetworkHVSock vsock.HVSockRegistryEntry
	// MountVsocks contains the currently-active vsocks, mapped to the
	// directory they should be mounted on.
	MountVsocks map[string]uint64
}

type WSLConfig struct {
	wslstuff *aThing
}

// Stubs
type QEMUConfig struct{}
type AppleHVConfig struct{}

func getHostUID() int {
	return 1000
}
