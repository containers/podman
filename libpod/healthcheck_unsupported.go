// +build !linux

package libpod

import "github.com/containers/libpod/libpod/define"

// createTimer systemd timers for healthchecks of a container
func (c *Container) createTimer() error {
	return define.ErrNotImplemented
}

// startTimer starts a systemd timer for the healthchecks
func (c *Container) startTimer() error {
	return define.ErrNotImplemented
}

// removeTimer removes the systemd timer and unit files
// for the container
func (c *Container) removeTimer() error {
	return define.ErrNotImplemented
}
