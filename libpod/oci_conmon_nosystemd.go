//go:build !remote && (linux || freebsd) && !systemd

package libpod

import (
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// addHealthCheckArgs adds healthcheck-related arguments to conmon for non-systemd builds
func (r *ConmonOCIRuntime) addHealthCheckArgs(ctr *Container, args []string) []string {
	// Add healthcheck configuration as CLI arguments if container has healthcheck config
	if ctr.HasHealthCheck() {
		healthConfig := ctr.HealthCheckConfig()
		if healthConfig != nil {
			logrus.Debugf("HEALTHCHECK: Adding healthcheck CLI args for container %s", ctr.ID())

			// Build healthcheck command and arguments from test array
			healthCmd, healthArgs := r.buildHealthcheckCmdAndArgs(healthConfig.Test)
			if healthCmd != "" {
				args = append(args, "--healthcheck-cmd", healthCmd)

				// Add all healthcheck arguments
				for _, arg := range healthArgs {
					args = append(args, "--healthcheck-arg", arg)
				}

				// Add optional healthcheck parameters with validation and defaults
				interval := r.validateAndGetInterval(healthConfig.Interval)
				timeout := r.validateAndGetTimeout(healthConfig.Timeout)
				retries := r.validateAndGetRetries(healthConfig.Retries)
				startPeriod := r.validateAndGetStartPeriod(healthConfig.StartPeriod)

				args = append(args, "--healthcheck-interval", strconv.Itoa(interval))
				args = append(args, "--healthcheck-timeout", strconv.Itoa(timeout))
				args = append(args, "--healthcheck-retries", strconv.Itoa(retries))
				args = append(args, "--healthcheck-start-period", strconv.Itoa(startPeriod))

				logrus.Debugf("HEALTHCHECK: Added healthcheck args for container %s: cmd=%s, args=%v, interval=%ds, timeout=%ds, retries=%d, start-period=%ds",
					ctr.ID(), healthCmd, healthArgs, interval, timeout, retries, startPeriod)
			} else {
				logrus.Warnf("HEALTHCHECK: Container %s has healthcheck config but no valid command", ctr.ID())
			}
		}
	} else {
		logrus.Debugf("HEALTHCHECK: Container %s does not have healthcheck config, skipping healthcheck args", ctr.ID())
	}
	return args
}

// buildHealthcheckCmdAndArgs converts Podman's healthcheck test array to command and arguments
func (r *ConmonOCIRuntime) buildHealthcheckCmdAndArgs(test []string) (string, []string) {
	if len(test) == 0 {
		return "", nil
	}

	// Handle special cases
	switch test[0] {
	case "", "NONE":
		return "", nil
	case "CMD":
		// CMD format: ["CMD", "curl", "-f", "http://localhost:8080/health"]
		// -> cmd="curl", args=["-f", "http://localhost:8080/health"]
		if len(test) > 1 {
			return test[1], test[2:]
		}
		return "", nil
	case "CMD-SHELL":
		// CMD-SHELL format: ["CMD-SHELL", "curl -f http://localhost:8080/health"]
		// -> cmd="/bin/sh", args=["-c", "curl -f http://localhost:8080/health"]
		if len(test) > 1 {
			return "/bin/sh", []string{"-c", test[1]}
		}
		return "", nil
	default:
		// Direct command format: ["curl", "-f", "http://localhost:8080/health"]
		// -> cmd="curl", args=["-f", "http://localhost:8080/health"]
		return test[0], test[1:]
	}
}

// validateAndGetInterval validates and returns the healthcheck interval in seconds
func (r *ConmonOCIRuntime) validateAndGetInterval(interval time.Duration) int {
	// Default interval is 30 seconds
	if interval <= 0 {
		return 30
	}
	// Ensure minimum interval of 1 second
	if interval < time.Second {
		logrus.Warnf("HEALTHCHECK: Interval %v is less than 1 second, using 1 second", interval)
		return 1
	}
	return int(interval.Seconds())
}

// validateAndGetTimeout validates and returns the healthcheck timeout in seconds
func (r *ConmonOCIRuntime) validateAndGetTimeout(timeout time.Duration) int {
	// Default timeout is 30 seconds
	if timeout <= 0 {
		return 30
	}
	// Ensure minimum timeout of 1 second
	if timeout < time.Second {
		logrus.Warnf("HEALTHCHECK: Timeout %v is less than 1 second, using 1 second", timeout)
		return 1
	}
	return int(timeout.Seconds())
}

// validateAndGetRetries validates and returns the healthcheck retries count
func (r *ConmonOCIRuntime) validateAndGetRetries(retries int) int {
	// Default retries is 3
	if retries <= 0 {
		return 3
	}
	// Ensure reasonable maximum retries (conmon should handle this too)
	if retries > 10 {
		logrus.Warnf("HEALTHCHECK: Retries %d is very high, using 10", retries)
		return 10
	}
	return retries
}

// validateAndGetStartPeriod validates and returns the healthcheck start period in seconds
func (r *ConmonOCIRuntime) validateAndGetStartPeriod(startPeriod time.Duration) int {
	// Default start period is 0 seconds
	if startPeriod < 0 {
		logrus.Warnf("HEALTHCHECK: Start period %v is negative, using 0", startPeriod)
		return 0
	}
	return int(startPeriod.Seconds())
}
