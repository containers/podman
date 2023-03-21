package libpod

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	// MaxHealthCheckNumberLogs is the maximum number of attempts we keep
	// in the healthcheck history file
	MaxHealthCheckNumberLogs int = 5
	// MaxHealthCheckLogLength in characters
	MaxHealthCheckLogLength = 500
)

// HealthCheck verifies the state and validity of the healthcheck configuration
// on the container and then executes the healthcheck
func (r *Runtime) HealthCheck(ctx context.Context, name string) (define.HealthCheckStatus, error) {
	container, err := r.LookupContainer(name)
	if err != nil {
		return define.HealthCheckContainerNotFound, fmt.Errorf("unable to look up %s to perform a health check: %w", name, err)
	}

	hcStatus, err := checkHealthCheckCanBeRun(container)
	if err != nil {
		return hcStatus, err
	}

	isStartupHC := false
	if container.config.StartupHealthCheckConfig != nil {
		passed, err := container.StartupHCPassed()
		if err != nil {
			return define.HealthCheckInternalError, err
		}
		isStartupHC = !passed
	}

	hcStatus, logStatus, err := container.runHealthCheck(ctx, isStartupHC)
	if !isStartupHC {
		if err := container.processHealthCheckStatus(logStatus); err != nil {
			return hcStatus, err
		}
	}
	return hcStatus, err
}

func (c *Container) runHealthCheck(ctx context.Context, isStartup bool) (define.HealthCheckStatus, string, error) {
	var (
		newCommand    []string
		returnCode    int
		inStartPeriod bool
	)
	hcCommand := c.HealthCheckConfig().Test
	if isStartup {
		logrus.Debugf("Running startup healthcheck for container %s", c.ID())
		hcCommand = c.config.StartupHealthCheckConfig.Test
	}
	if len(hcCommand) < 1 {
		return define.HealthCheckNotDefined, "", fmt.Errorf("container %s has no defined healthcheck", c.ID())
	}
	switch hcCommand[0] {
	case "", define.HealthConfigTestNone:
		return define.HealthCheckNotDefined, "", fmt.Errorf("container %s has no defined healthcheck", c.ID())
	case define.HealthConfigTestCmd:
		newCommand = hcCommand[1:]
	case define.HealthConfigTestCmdShell:
		// TODO: SHELL command from image not available in Container - use Docker default
		newCommand = []string{"/bin/sh", "-c", strings.Join(hcCommand[1:], " ")}
	default:
		// command supplied on command line - pass as-is
		newCommand = hcCommand
	}
	if len(newCommand) < 1 || newCommand[0] == "" {
		return define.HealthCheckNotDefined, "", fmt.Errorf("container %s has no defined healthcheck", c.ID())
	}
	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		return define.HealthCheckInternalError, "", fmt.Errorf("unable to create pipe for healthcheck session: %w", err)
	}
	defer wPipe.Close()
	defer rPipe.Close()

	streams := new(define.AttachStreams)

	streams.InputStream = bufio.NewReader(os.Stdin)
	streams.OutputStream = wPipe
	streams.ErrorStream = wPipe
	streams.AttachOutput = true
	streams.AttachError = true
	streams.AttachInput = true

	stdout := []string{}
	go func() {
		scanner := bufio.NewScanner(rPipe)
		for scanner.Scan() {
			stdout = append(stdout, scanner.Text())
		}
	}()

	logrus.Debugf("executing health check command %s for %s", strings.Join(newCommand, " "), c.ID())
	timeStart := time.Now()
	hcResult := define.HealthCheckSuccess
	config := new(ExecConfig)
	config.Command = newCommand
	exitCode, hcErr := c.exec(config, streams, nil, true)
	if hcErr != nil {
		hcResult = define.HealthCheckFailure
		if errors.Is(hcErr, define.ErrOCIRuntimeNotFound) ||
			errors.Is(hcErr, define.ErrOCIRuntimePermissionDenied) ||
			errors.Is(hcErr, define.ErrOCIRuntime) {
			returnCode = 1
			hcErr = nil
		} else {
			returnCode = 125
		}
	} else if exitCode != 0 {
		hcResult = define.HealthCheckFailure
		returnCode = 1
	}

	// Handle startup HC
	if isStartup {
		inStartPeriod = true
		if hcErr != nil || exitCode != 0 {
			hcResult = define.HealthCheckStartup
			c.incrementStartupHCFailureCounter(ctx)
		} else {
			c.incrementStartupHCSuccessCounter(ctx)
		}
	}

	timeEnd := time.Now()
	if c.HealthCheckConfig().StartPeriod > 0 {
		// there is a start-period we need to honor; we add startPeriod to container start time
		startPeriodTime := c.state.StartedTime.Add(c.HealthCheckConfig().StartPeriod)
		if timeStart.Before(startPeriodTime) {
			// we are still in the start period, flip the inStartPeriod bool
			inStartPeriod = true
			logrus.Debugf("healthcheck for %s being run in start-period", c.ID())
		}
	}

	eventLog := strings.Join(stdout, "\n")
	if len(eventLog) > MaxHealthCheckLogLength {
		eventLog = eventLog[:MaxHealthCheckLogLength]
	}

	if timeEnd.Sub(timeStart) > c.HealthCheckConfig().Timeout {
		returnCode = -1
		hcResult = define.HealthCheckFailure
		hcErr = fmt.Errorf("healthcheck command exceeded timeout of %s", c.HealthCheckConfig().Timeout.String())
	}

	hcl := newHealthCheckLog(timeStart, timeEnd, returnCode, eventLog)
	logStatus, err := c.updateHealthCheckLog(hcl, inStartPeriod)
	if err != nil {
		return hcResult, "", fmt.Errorf("unable to update health check log %s for %s: %w", c.healthCheckLogPath(), c.ID(), err)
	}

	return hcResult, logStatus, hcErr
}

func (c *Container) processHealthCheckStatus(status string) error {
	if status != define.HealthCheckUnhealthy {
		return nil
	}

	switch c.config.HealthCheckOnFailureAction {
	case define.HealthCheckOnFailureActionNone: // Nothing to do

	case define.HealthCheckOnFailureActionKill:
		if err := c.Kill(uint(unix.SIGKILL)); err != nil {
			return fmt.Errorf("killing container health-check turned unhealthy: %w", err)
		}

	case define.HealthCheckOnFailureActionRestart:
		// We let the cleanup process handle the restart.  Otherwise
		// the container would be restarted in the context of a
		// transient systemd unit which may cause undesired side
		// effects.
		if err := c.Stop(); err != nil {
			return fmt.Errorf("restarting/stopping container after health-check turned unhealthy: %w", err)
		}

	case define.HealthCheckOnFailureActionStop:
		if err := c.Stop(); err != nil {
			return fmt.Errorf("stopping container after health-check turned unhealthy: %w", err)
		}

	default: // Should not happen but better be safe than sorry
		return fmt.Errorf("unsupported on-failure action %d", c.config.HealthCheckOnFailureAction)
	}

	return nil
}

func checkHealthCheckCanBeRun(c *Container) (define.HealthCheckStatus, error) {
	cstate, err := c.State()
	if err != nil {
		return define.HealthCheckInternalError, err
	}
	if cstate != define.ContainerStateRunning {
		return define.HealthCheckContainerStopped, fmt.Errorf("container %s is not running", c.ID())
	}
	if !c.HasHealthCheck() {
		return define.HealthCheckNotDefined, fmt.Errorf("container %s has no defined healthcheck", c.ID())
	}
	return define.HealthCheckDefined, nil
}

// Increment the current startup healthcheck success counter.
// Can stop the startup HC and start the regular HC if the startup HC has enough
// consecutive successes.
func (c *Container) incrementStartupHCSuccessCounter(ctx context.Context) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			logrus.Errorf("Error syncing container %s state: %v", c.ID(), err)
			return
		}
	}

	// We don't have a startup HC, can't do anything
	if c.config.StartupHealthCheckConfig == nil {
		return
	}

	// Race: someone else got here first
	if c.state.StartupHCPassed {
		return
	}

	// Increment the success counter
	c.state.StartupHCSuccessCount++

	logrus.Debugf("Startup healthcheck for container %s succeeded, success counter now %d", c.ID(), c.state.StartupHCSuccessCount)

	// Did we exceed threshold?
	recreateTimer := false
	if c.config.StartupHealthCheckConfig.Successes == 0 || c.state.StartupHCSuccessCount >= c.config.StartupHealthCheckConfig.Successes {
		c.state.StartupHCPassed = true
		c.state.StartupHCSuccessCount = 0
		c.state.StartupHCFailureCount = 0

		recreateTimer = true
	}

	if err := c.save(); err != nil {
		logrus.Errorf("Error saving container %s state: %v", c.ID(), err)
		return
	}

	if recreateTimer {
		logrus.Infof("Startup healthcheck for container %s passed, recreating timer", c.ID())

		// Create the new, standard healthcheck timer first.
		if err := c.createTimer(c.HealthCheckConfig().Interval.String(), false); err != nil {
			logrus.Errorf("Error recreating container %s healthcheck: %v", c.ID(), err)
			return
		}
		if err := c.startTimer(false); err != nil {
			logrus.Errorf("Error restarting container %s healthcheck timer: %v", c.ID(), err)
		}

		// This kills the process the healthcheck is running.
		// Which happens to be us.
		// So this has to be last - after this, systemd serves us a
		// SIGTERM and we exit.
		if err := c.removeTransientFiles(ctx, true); err != nil {
			logrus.Errorf("Error removing container %s healthcheck: %v", c.ID(), err)
			return
		}
	}
}

// Increment the current startup healthcheck failure counter.
// Can restart the container if the HC fails enough times consecutively.
func (c *Container) incrementStartupHCFailureCounter(ctx context.Context) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			logrus.Errorf("Error syncing container %s state: %v", c.ID(), err)
			return
		}
	}

	// We don't have a startup HC, can't do anything
	if c.config.StartupHealthCheckConfig == nil {
		return
	}

	// Race: someone else got here first
	if c.state.StartupHCPassed {
		return
	}

	c.state.StartupHCFailureCount++

	logrus.Debugf("Startup healthcheck for container %s failed, failure counter now %d", c.ID(), c.state.StartupHCFailureCount)

	if c.config.StartupHealthCheckConfig.Retries != 0 && c.state.StartupHCFailureCount >= c.config.StartupHealthCheckConfig.Retries {
		logrus.Infof("Restarting container %s as startup healthcheck failed", c.ID())
		// Restart the container
		if err := c.restartWithTimeout(ctx, c.config.StopTimeout); err != nil {
			logrus.Errorf("Error restarting container %s after healthcheck failure: %v", c.ID(), err)
		}
		return
	}

	if err := c.save(); err != nil {
		logrus.Errorf("Error saving container %s state: %v", c.ID(), err)
	}
}

func newHealthCheckLog(start, end time.Time, exitCode int, log string) define.HealthCheckLog {
	return define.HealthCheckLog{
		Start:    start.Format(time.RFC3339Nano),
		End:      end.Format(time.RFC3339Nano),
		ExitCode: exitCode,
		Output:   log,
	}
}

// updatedHealthCheckStatus updates the health status of the container
// in the healthcheck log
func (c *Container) updateHealthStatus(status string) error {
	healthCheck, err := c.getHealthCheckLog()
	if err != nil {
		return err
	}
	healthCheck.Status = status
	newResults, err := json.Marshal(healthCheck)
	if err != nil {
		return fmt.Errorf("unable to marshall healthchecks for writing status: %w", err)
	}
	return os.WriteFile(c.healthCheckLogPath(), newResults, 0700)
}

// isUnhealthy returns if the current health check status in unhealthy.
func (c *Container) isUnhealthy() (bool, error) {
	if !c.HasHealthCheck() {
		return false, nil
	}
	healthCheck, err := c.getHealthCheckLog()
	if err != nil {
		return false, err
	}
	return healthCheck.Status == define.HealthCheckUnhealthy, nil
}

// UpdateHealthCheckLog parses the health check results and writes the log
func (c *Container) updateHealthCheckLog(hcl define.HealthCheckLog, inStartPeriod bool) (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	healthCheck, err := c.getHealthCheckLog()
	if err != nil {
		return "", err
	}
	if hcl.ExitCode == 0 {
		//	set status to healthy, reset failing state to 0
		healthCheck.Status = define.HealthCheckHealthy
		healthCheck.FailingStreak = 0
	} else {
		if len(healthCheck.Status) < 1 {
			healthCheck.Status = define.HealthCheckHealthy
		}
		if !inStartPeriod {
			// increment failing streak
			healthCheck.FailingStreak++
			// if failing streak > retries, then status to unhealthy
			if healthCheck.FailingStreak >= c.HealthCheckConfig().Retries {
				healthCheck.Status = define.HealthCheckUnhealthy
			}
		}
	}
	healthCheck.Log = append(healthCheck.Log, hcl)
	if len(healthCheck.Log) > MaxHealthCheckNumberLogs {
		healthCheck.Log = healthCheck.Log[1:]
	}
	newResults, err := json.Marshal(healthCheck)
	if err != nil {
		return "", fmt.Errorf("unable to marshall healthchecks for writing: %w", err)
	}
	return healthCheck.Status, os.WriteFile(c.healthCheckLogPath(), newResults, 0700)
}

// HealthCheckLogPath returns the path for where the health check log is
func (c *Container) healthCheckLogPath() string {
	return filepath.Join(filepath.Dir(c.state.RunDir), "healthcheck.log")
}

// getHealthCheckLog returns HealthCheck results by reading the container's
// health check log file.  If the health check log file does not exist, then
// an empty healthcheck struct is returned
// The caller should lock the container before this function is called.
func (c *Container) getHealthCheckLog() (define.HealthCheckResults, error) {
	var healthCheck define.HealthCheckResults
	if _, err := os.Stat(c.healthCheckLogPath()); os.IsNotExist(err) {
		return healthCheck, nil
	}
	b, err := os.ReadFile(c.healthCheckLogPath())
	if err != nil {
		return healthCheck, fmt.Errorf("failed to read health check log file: %w", err)
	}
	if err := json.Unmarshal(b, &healthCheck); err != nil {
		return healthCheck, fmt.Errorf("failed to unmarshal existing healthcheck results in %s: %w", c.healthCheckLogPath(), err)
	}
	return healthCheck, nil
}

// HealthCheckStatus returns the current state of a container with a healthcheck.
// Returns an empty string if no health check is defined for the container.
func (c *Container) HealthCheckStatus() (string, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()
	}
	return c.healthCheckStatus()
}

// Internal function to return the current state of a container with a healthcheck.
// This function does not lock the container.
func (c *Container) healthCheckStatus() (string, error) {
	if !c.HasHealthCheck() {
		return "", nil
	}

	if err := c.syncContainer(); err != nil {
		return "", err
	}

	results, err := c.getHealthCheckLog()
	if err != nil {
		return "", fmt.Errorf("unable to get healthcheck log for %s: %w", c.ID(), err)
	}

	return results.Status, nil
}
