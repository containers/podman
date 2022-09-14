//go:build !linux && !freebsd
// +build !linux,!freebsd

package libpod

import (
	"errors"

	"github.com/containers/podman/v4/libpod/define"
)

// GetContainerStats gets the running stats for a given container.
// The previousStats is used to correctly calculate cpu percentages. You
// should pass nil if there is no previous stat for this container.
func (c *Container) GetContainerStats(previousStats *define.ContainerStats) (*define.ContainerStats, error) {
	return nil, errors.New("not implemented (*Container) GetContainerStats")
}
