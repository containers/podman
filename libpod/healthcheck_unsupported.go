//go:build !remote && !linux

package libpod

import (
	"context"
)

// ReattachHealthCheckTimers reattaches healthcheck timers for running containers after podman restart
// This is a no-op for unsupported platforms since healthchecks are not supported
func ReattachHealthCheckTimers(containers []*Container) {
	// Healthchecks are not supported on this platform
}

// createTimer systemd timers for healthchecks of a container
func (c *Container) createTimer(interval string, isStartup bool) error {
	return nil
}

// startTimer starts a systemd timer for the healthchecks
func (c *Container) startTimer(isStartup bool) error {
	return nil
}

// removeTransientFiles removes the systemd timer and unit files
// for the container
func (c *Container) removeTransientFiles(ctx context.Context, isStartup bool, unitName string) error {
	return nil
}
