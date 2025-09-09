//go:build !remote && !systemd

package libpod

import (
	"context"
	"fmt"
	"time"

	"github.com/containers/podman/v5/libpod/define"
	"github.com/sirupsen/logrus"
)

// healthcheckTimer manages the background goroutine for healthchecks
type healthcheckTimer struct {
	container *Container
	interval  time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
}

// Global map to track active timers (in a real implementation, this would be part of the runtime)
var activeTimers = make(map[string]*healthcheckTimer)

// disableHealthCheckSystemd returns true if healthcheck should be disabled
// For non-systemd builds, we only disable if interval is 0
func (c *Container) disableHealthCheckSystemd(isStartup bool) bool {
	if isStartup {
		if c.config.StartupHealthCheckConfig != nil && c.config.StartupHealthCheckConfig.Interval == 0 {
			return true
		}
	}
	if c.config.HealthCheckConfig != nil && c.config.HealthCheckConfig.Interval == 0 {
		return true
	}
	return false
}

// createTimer creates a goroutine-based timer for healthchecks of a container
func (c *Container) createTimer(interval string, isStartup bool) error {
	if c.disableHealthCheckSystemd(isStartup) {
		return nil
	}

	// Parse the interval duration
	duration, err := time.ParseDuration(interval)
	if err != nil {
		return err
	}

	// Stop any existing timer
	if c.state.HCUnitName != "" {
		c.stopHealthCheckTimer()
	}

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Create timer struct
	timer := &healthcheckTimer{
		container: c,
		interval:  duration,
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	// Store timer reference globally and in container state
	activeTimers[c.ID()] = timer
	c.state.HCUnitName = "goroutine-timer"

	if err := c.save(); err != nil {
		cancel()
		delete(activeTimers, c.ID())
		return fmt.Errorf("saving container %s healthcheck timer: %w", c.ID(), err)
	}

	// Start the background goroutine
	go timer.run()

	logrus.Debugf("Created goroutine-based healthcheck timer for container %s with interval %s", c.ID(), interval)
	return nil
}

// startTimer starts the goroutine-based timer for healthchecks
func (c *Container) startTimer(isStartup bool) error {
	// Timer is already started in createTimer, nothing to do
	return nil
}

// removeTransientFiles stops the goroutine-based timer
func (c *Container) removeTransientFiles(ctx context.Context, isStartup bool, unitName string) error {
	return c.stopHealthCheckTimer()
}

// stopHealthCheckTimer stops the background healthcheck goroutine
func (c *Container) stopHealthCheckTimer() error {
	timer, exists := activeTimers[c.ID()]
	if !exists {
		logrus.Debugf("No active healthcheck timer found for container %s", c.ID())
		return nil
	}

	logrus.Debugf("Stopping healthcheck timer for container %s", c.ID())

	// Cancel the context to stop the goroutine
	timer.cancel()

	// Wait for the goroutine to finish (with timeout)
	select {
	case <-timer.done:
		logrus.Debugf("Healthcheck timer for container %s stopped gracefully", c.ID())
	case <-time.After(5 * time.Second):
		logrus.Warnf("Healthcheck timer for container %s did not stop within timeout", c.ID())
	}

	// Remove from active timers
	delete(activeTimers, c.ID())

	// Clear the unit name
	c.state.HCUnitName = ""
	return c.save()
}

// run executes the healthcheck in a loop with the specified interval
func (t *healthcheckTimer) run() {
	defer close(t.done)

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	logrus.Debugf("Starting healthcheck timer for container %s with interval %s", t.container.ID(), t.interval)

	for {
		select {
		case <-t.ctx.Done():
			logrus.Debugf("Healthcheck timer for container %s stopped", t.container.ID())
			return
		case <-ticker.C:
			// Run the healthcheck
			if err := t.runHealthCheck(); err != nil {
				logrus.Errorf("Healthcheck failed for container %s: %v", t.container.ID(), err)
			}
		}
	}
}

// runHealthCheck executes a single healthcheck
func (t *healthcheckTimer) runHealthCheck() error {
	// Check if container is still running (without holding lock to avoid deadlock)
	state, err := t.container.State()
	if err != nil {
		return err
	}

	if state != define.ContainerStateRunning {
		logrus.Debugf("Container %s is not running (state: %v), skipping healthcheck", t.container.ID(), state)
		return nil
	}

	// Get healthcheck config (without holding lock)
	healthConfig := t.container.HealthCheckConfig()
	if healthConfig == nil {
		logrus.Debugf("No healthcheck config found for container %s, skipping healthcheck", t.container.ID())
		return nil
	}

	// Run the healthcheck - let runHealthCheck handle its own locking internally
	ctx, cancel := context.WithTimeout(context.Background(), healthConfig.Timeout)
	defer cancel()

	_, _, err = t.container.runHealthCheck(ctx, false)
	return err
}
