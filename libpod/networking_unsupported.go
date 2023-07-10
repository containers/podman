//go:build !linux && !freebsd
// +build !linux,!freebsd

package libpod

import (
	"errors"
	"net"
	"path/filepath"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/storage/pkg/lockfile"
)

type RootlessNetNS struct {
	dir  string
	Lock *lockfile.LockFile
}

// ocicniPortsToNetTypesPorts convert the old port format to the new one
// while deduplicating ports into ranges
func ocicniPortsToNetTypesPorts(ports []types.OCICNIPortMapping) []types.PortMapping {
	return []types.PortMapping{}
}

func (c *Container) getContainerNetworkInfo() (*define.InspectNetworkSettings, error) {
	return nil, errors.New("not implemented (*Container) getContainerNetworkInfo")
}

func (c *Container) setupRootlessNetwork() error {
	return errors.New("not implemented (*Container) setupRootlessNetwork")
}

func (r *Runtime) setupNetNS(ctr *Container) error {
	return errors.New("not implemented (*Runtime) setupNetNS")
}

// normalizeNetworkName takes a network name, a partial or a full network ID and returns the network name.
// If the network is not found an error is returned.
func (r *Runtime) normalizeNetworkName(nameOrID string) (string, error) {
	return "", errors.New("not implemented (*Runtime) normalizeNetworkName")
}

// DisconnectContainerFromNetwork removes a container from its network
func (r *Runtime) DisconnectContainerFromNetwork(nameOrID, netName string, force bool) error {
	return errors.New("not implemented (*Runtime) DisconnectContainerFromNetwork")
}

// ConnectContainerToNetwork connects a container to a network
func (r *Runtime) ConnectContainerToNetwork(nameOrID, netName string, netOpts types.PerNetworkOptions) error {
	return errors.New("not implemented (*Runtime) ConnectContainerToNetwork")
}

// getPath will join the given path to the rootless netns dir
func (r *RootlessNetNS) getPath(path string) string {
	return filepath.Join(r.dir, path)
}

// Do - run the given function in the rootless netns.
// It does not lock the rootlessCNI lock, the caller
// should only lock when needed, e.g. for network operations.
func (r *RootlessNetNS) Do(toRun func() error) error {
	return errors.New("not implemented (*RootlessNetNS) Do")
}

// Cleanup the rootless network namespace if needed.
// It checks if we have running containers with the bridge network mode.
// Cleanup() expects that r.Lock is locked
func (r *RootlessNetNS) Cleanup(runtime *Runtime) error {
	return errors.New("not implemented (*RootlessNetNS) Cleanup")
}

// GetRootlessNetNs returns the rootless netns object. If create is set to true
// the rootless network namespace will be created if it does not already exist.
// If called as root it returns always nil.
// On success the returned RootlessCNI lock is locked and must be unlocked by the caller.
func (r *Runtime) GetRootlessNetNs(new bool) (*RootlessNetNS, error) {
	return nil, errors.New("not implemented (*Runtime) GetRootlessNetNs")
}

// convertPortMappings will remove the HostIP part from the ports when running inside podman machine.
// This is need because a HostIP of 127.0.0.1 would now allow the gvproxy forwarder to reach to open ports.
// For machine the HostIP must only be used by gvproxy and never in the VM.
func (c *Container) convertPortMappings() []types.PortMapping {
	return []types.PortMapping{}
}
