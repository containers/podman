//go:build !linux
// +build !linux

package libpod

import (
	"context"
	"errors"
)

// createTimer systemd timers for healthchecks of a container
func (c *Container) createTimer() error {
	return errors.New("not implemented (*Container) createTimer")
}

// startTimer starts a systemd timer for the healthchecks
func (c *Container) startTimer() error {
	return errors.New("not implemented (*Container) startTimer")
}

// removeTransientFiles removes the systemd timer and unit files
// for the container
func (c *Container) removeTransientFiles(ctx context.Context) error {
	return errors.New("not implemented (*Container) removeTransientFiles")
}
