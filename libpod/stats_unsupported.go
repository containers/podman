// +build !linux

package libpod

import "github.com/containers/libpod/libpod/define"

// GetContainerStats gets the running stats for a given container
func (c *Container) GetContainerStats(previousStats *ContainerStats) (*ContainerStats, error) {
	return nil, define.ErrOSNotSupported
}
