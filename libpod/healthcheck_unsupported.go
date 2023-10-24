//go:build !remote && !linux
// +build !remote,!linux

package libpod

import (
	"context"
)

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
func (c *Container) removeTransientFiles(ctx context.Context, isStartup bool) error {
	return nil
}
