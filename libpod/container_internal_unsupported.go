//go:build !linux && !freebsd
// +build !linux,!freebsd

package libpod

import (
	"context"
	"errors"

	"github.com/containers/common/libnetwork/etchosts"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/lookup"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func (c *Container) mountSHM(shmOptions string) error {
	return errors.New("not implemented (*Container) mountSHM")
}

func (c *Container) unmountSHM(mount string) error {
	return errors.New("not implemented (*Container) unmountSHM")
}

func (c *Container) cleanupOverlayMounts() error {
	return errors.New("not implemented (*Container) cleanupOverlayMounts")
}

// prepare mounts the container and sets up other required resources like net
// namespaces
func (c *Container) prepare() error {
	return errors.New("not implemented (*Container) prepare")
}

// resolveWorkDir resolves the container's workdir and, depending on the
// configuration, will create it, or error out if it does not exist.
// Note that the container must be mounted before.
func (c *Container) resolveWorkDir() error {
	return errors.New("not implemented (*Container) resolveWorkDir")
}

// cleanupNetwork unmounts and cleans up the container's network
func (c *Container) cleanupNetwork() error {
	return errors.New("not implemented (*Container) cleanupNetwork")
}

// reloadNetwork reloads the network for the given container, recreating
// firewall rules.
func (c *Container) reloadNetwork() error {
	return errors.New("not implemented (*Container) reloadNetwork")
}

// Generate spec for a container
// Accepts a map of the container's dependencies
func (c *Container) generateSpec(ctx context.Context) (*spec.Spec, error) {
	return nil, errors.New("not implemented (*Container) generateSpec")
}

func (c *Container) getUserOverrides() *lookup.Overrides {
	return &lookup.Overrides{}
}

func (c *Container) checkpoint(ctx context.Context, options ContainerCheckpointOptions) (*define.CRIUCheckpointRestoreStatistics, int64, error) {
	return nil, 0, errors.New("not implemented (*Container) checkpoint")
}

func (c *Container) restore(ctx context.Context, options ContainerCheckpointOptions) (criuStatistics *define.CRIUCheckpointRestoreStatistics, runtimeRestoreDuration int64, retErr error) {
	return nil, 0, errors.New("not implemented (*Container) restore")
}

// getHostsEntries returns the container ip host entries for the correct netmode
func (c *Container) getHostsEntries() (etchosts.HostEntries, error) {
	return nil, errors.New("unsupported (*Container) getHostsEntries")
}

// Fix ownership and permissions of the specified volume if necessary.
func (c *Container) fixVolumePermissions(v *ContainerNamedVolume) error {
	return errors.New("unsupported (*Container) fixVolumePermissions")
}

func (c *Container) expectPodCgroup() (bool, error) {
	return false, errors.New("unsupported (*Container) expectPodCgroup")
}

// Get cgroup path in a format suitable for the OCI spec
func (c *Container) getOCICgroupPath() (string, error) {
	return "", errors.New("unsupported (*Container) getOCICgroupPath")
}

func getLocalhostHostEntry(c *Container) etchosts.HostEntries {
	return nil
}

func isRootlessCgroupSet(cgroup string) bool {
	return false
}

func openDirectory(path string) (fd int, err error) {
	return -1, errors.New("unsupported openDirectory")
}
