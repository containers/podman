// +build !linux

package libpod

// createTimer systemd timers for healthchecks of a container
func (c *Container) createTimer() error {
	return ErrNotImplemented
}

// startTimer starts a systemd timer for the healthchecks
func (c *Container) startTimer() error {
	return ErrNotImplemented
}

// removeTimer removes the systemd timer and unit files
// for the container
func (c *Container) removeTimer() error {
	return ErrNotImplemented
}
