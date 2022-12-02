//go:build !linux
// +build !linux

package libpod

import (
	"context"
	"errors"
)

// createTimer systemd timers for healthchecks of a container
func (c *Container) createTimer(interval string, isStartup bool) error {
	return errors.New("not implemented (*Container) createTimer")
}

// startTimer starts a systemd timer for the healthchecks
func (c *Container) startTimer(isStartup bool) error {
	return errors.New("not implemented (*Container) startTimer")
}

// removeTransientFiles removes the systemd timer and unit files
// for the container
func (c *Container) removeTransientFiles(ctx context.Context, isStartup bool) error {
	return errors.New("not implemented (*Container) removeTransientFiles")
}
