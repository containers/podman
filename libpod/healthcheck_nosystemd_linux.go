//go:build !remote && !systemd

package libpod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// ReattachHealthCheckTimers reattaches healthcheck timers for running containers after podman restart
// This implementation is for nosystemd builds where healthchecks are managed by goroutines
func ReattachHealthCheckTimers(containers []*Container) {
	for _, ctr := range containers {
		// Only reattach for running containers with healthcheck configs
		if ctr.state.State != define.ContainerStateRunning {
			continue
		}

		// Check if container has healthcheck config
		if ctr.config.HealthCheckConfig == nil {
			continue
		}

		// Check if timer is already running
		if _, exists := activeTimers[ctr.ID()]; exists {
			continue
		}

		// Check if this is a startup healthcheck that hasn't passed yet
		if ctr.config.StartupHealthCheckConfig != nil && !ctr.state.StartupHCPassed {
			// Reattach startup healthcheck
			interval := ctr.config.StartupHealthCheckConfig.StartInterval.String()
			if err := ctr.createTimer(interval, true); err != nil {
				logrus.Errorf("Failed to reattach startup healthcheck timer for container %s: %v", ctr.ID(), err)
			}
		} else if ctr.state.StartupHCPassed || ctr.config.StartupHealthCheckConfig == nil {
			// Reattach regular healthcheck
			interval := ctr.config.HealthCheckConfig.Interval.String()
			if err := ctr.createTimer(interval, false); err != nil {
				logrus.Errorf("Failed to reattach healthcheck timer for container %s: %v", ctr.ID(), err)
			}
		}
	}
}

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

	// Stop any existing timer only if there's actually an active timer in memory
	if c.state.HCUnitName != "" {
		// Check if there's an active timer in memory before stopping
		if _, exists := activeTimers[c.ID()]; exists {
			c.stopHealthCheckTimer()
		} else {
			// No active timer in memory, just clear the state without creating stop file
			c.state.HCUnitName = ""
			c.state.HealthCheckStopFile = ""
			if err := c.save(); err != nil {
				return fmt.Errorf("clearing container %s healthcheck state: %w", c.ID(), err)
			}
		}
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
	// Create a stop file for cross-process cleanup
	stopFile := filepath.Join(c.runtime.config.Engine.TmpDir, fmt.Sprintf("healthcheck-stop-%s", c.ID()))
	c.state.HealthCheckStopFile = stopFile

	if err := c.save(); err != nil {
		cancel()
		delete(activeTimers, c.ID())
		return fmt.Errorf("saving container %s healthcheck timer: %w", c.ID(), err)
	}

	// Start the background goroutine
	go timer.run()

	return nil
}

// startTimer starts the goroutine-based timer for healthchecks
func (c *Container) startTimer(isStartup bool) error {
	// Check if timer already exists
	if _, exists := activeTimers[c.ID()]; exists {
		return nil
	}

	// Create timer if it doesn't exist
	if c.config.HealthCheckConfig != nil {
		interval := c.config.HealthCheckConfig.Interval.String()
		if c.config.StartupHealthCheckConfig != nil && !c.state.StartupHCPassed {
			interval = c.config.StartupHealthCheckConfig.StartInterval.String()
		}
		return c.createTimer(interval, c.config.StartupHealthCheckConfig != nil)
	}

	return nil
}

// removeTransientFiles stops the goroutine-based timer
func (c *Container) removeTransientFiles(ctx context.Context, isStartup bool, unitName string) error {
	return c.stopHealthCheckTimer()
}

// stopHealthCheckTimer stops the background healthcheck goroutine
func (c *Container) stopHealthCheckTimer() error {
	// First try to stop using the in-memory map (same process)
	timer, exists := activeTimers[c.ID()]
	if exists {
		// Cancel the context to stop the goroutine
		timer.cancel()

		// Wait for the goroutine to finish (with timeout)
		select {
		case <-timer.done:
			// Timer stopped gracefully
		case <-time.After(5 * time.Second):
			logrus.Warnf("Healthcheck timer for container %s did not stop within timeout", c.ID())
		}

		// Remove from active timers
		delete(activeTimers, c.ID())
	} else if c.state.HealthCheckStopFile != "" {
		// Called from different process (cleanup), create stop file
		if err := os.WriteFile(c.state.HealthCheckStopFile, []byte("stop"), 0644); err != nil {
			logrus.Errorf("Failed to create healthcheck stop file for container %s: %v", c.ID(), err)
		}
	}

	// Clear the unit name and stop file
	c.state.HCUnitName = ""
	c.state.HealthCheckStopFile = ""
	return c.save()
}

// run executes the healthcheck in a loop with the specified interval
func (t *healthcheckTimer) run() {
	defer close(t.done)

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			// Check for stop file (cross-process cleanup)
			if t.container.state.HealthCheckStopFile != "" {
				if _, err := os.Stat(t.container.state.HealthCheckStopFile); err == nil {
					// Clean up the stop file
					if err := os.Remove(t.container.state.HealthCheckStopFile); err != nil {
						logrus.Warnf("Failed to remove stop file for container %s: %v", t.container.ID(), err)
					}
					return
				}
			}

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
		return nil
	}

	// Get healthcheck config (without holding lock)
	healthConfig := t.container.HealthCheckConfig()
	if healthConfig == nil {
		return nil
	}

	// Run the healthcheck - let runHealthCheck handle its own locking internally
	ctx, cancel := context.WithTimeout(context.Background(), healthConfig.Timeout)
	defer cancel()

	_, _, err = t.container.runHealthCheck(ctx, false)
	return err
}
