// +build !linux

package libpod

import "github.com/containers/podman/v2/libpod/define"

// GetContainerStats gets the running stats for a given container
func (c *Container) GetContainerStats(previousStats *define.ContainerStats) (*define.ContainerStats, error) {
	return nil, define.ErrOSNotSupported
}
