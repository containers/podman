package libpod

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// MaxHealthCheckNumberLogs is the maximum number of attempts we keep
	// in the healthcheck history file
	MaxHealthCheckNumberLogs int = 5
	// MaxHealthCheckLogLength in characters
	MaxHealthCheckLogLength = 500
)

// hcWriteCloser allows us to use bufio as a WriteCloser
type hcWriteCloser struct {
	*bufio.Writer
}

// Used to add a closer to bufio
func (hcwc hcWriteCloser) Close() error {
	return nil
}

// HealthCheck verifies the state and validity of the healthcheck configuration
// on the container and then executes the healthcheck
func (r *Runtime) HealthCheck(name string) (define.HealthCheckStatus, error) {
	container, err := r.LookupContainer(name)
	if err != nil {
		return define.HealthCheckContainerNotFound, errors.Wrapf(err, "unable to lookup %s to perform a health check", name)
	}
	hcStatus, err := checkHealthCheckCanBeRun(container)
	if err == nil {
		return container.runHealthCheck()
	}
	return hcStatus, err
}

// runHealthCheck runs the health check as defined by the container
func (c *Container) runHealthCheck() (define.HealthCheckStatus, error) {
	var (
		newCommand    []string
		returnCode    int
		capture       bytes.Buffer
		inStartPeriod bool
	)
	hcCommand := c.HealthCheckConfig().Test
	if len(hcCommand) < 1 {
		return define.HealthCheckNotDefined, errors.Errorf("container %s has no defined healthcheck", c.ID())
	}
	switch hcCommand[0] {
	case "", "NONE":
		return define.HealthCheckNotDefined, errors.Errorf("container %s has no defined healthcheck", c.ID())
	case "CMD":
		newCommand = hcCommand[1:]
	case "CMD-SHELL":
		// TODO: SHELL command from image not available in Container - use Docker default
		newCommand = []string{"/bin/sh", "-c", strings.Join(hcCommand[1:], " ")}
	default:
		// command supplied on command line - pass as-is
		newCommand = hcCommand
	}
	if len(newCommand) < 1 || newCommand[0] == "" {
		return define.HealthCheckNotDefined, errors.Errorf("container %s has no defined healthcheck", c.ID())
	}
	captureBuffer := bufio.NewWriter(&capture)
	hcw := hcWriteCloser{
		captureBuffer,
	}
	streams := new(define.AttachStreams)
	streams.OutputStream = hcw
	streams.ErrorStream = hcw

	streams.InputStream = bufio.NewReader(os.Stdin)

	streams.AttachOutput = true
	streams.AttachError = true
	streams.AttachInput = true

	logrus.Debugf("executing health check command %s for %s", strings.Join(newCommand, " "), c.ID())
	timeStart := time.Now()
	hcResult := define.HealthCheckSuccess
	config := new(ExecConfig)
	config.Command = newCommand
	exitCode, hcErr := c.Exec(config, streams, nil)
	if hcErr != nil {
		errCause := errors.Cause(hcErr)
		hcResult = define.HealthCheckFailure
		if errCause == define.ErrOCIRuntimeNotFound ||
			errCause == define.ErrOCIRuntimePermissionDenied ||
			errCause == define.ErrOCIRuntime {
			returnCode = 1
			hcErr = nil
		} else {
			returnCode = 125
		}
	} else if exitCode != 0 {
		hcResult = define.HealthCheckFailure
		returnCode = 1
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

	eventLog := capture.String()
	if len(eventLog) > MaxHealthCheckLogLength {
		eventLog = eventLog[:MaxHealthCheckLogLength]
	}

	if timeEnd.Sub(timeStart) > c.HealthCheckConfig().Timeout {
		returnCode = -1
		hcResult = define.HealthCheckFailure
		hcErr = errors.Errorf("healthcheck command exceeded timeout of %s", c.HealthCheckConfig().Timeout.String())
	}
	hcl := newHealthCheckLog(timeStart, timeEnd, returnCode, eventLog)
	if err := c.updateHealthCheckLog(hcl, inStartPeriod); err != nil {
		return hcResult, errors.Wrapf(err, "unable to update health check log %s for %s", c.healthCheckLogPath(), c.ID())
	}
	return hcResult, hcErr
}

func checkHealthCheckCanBeRun(c *Container) (define.HealthCheckStatus, error) {
	cstate, err := c.State()
	if err != nil {
		return define.HealthCheckInternalError, err
	}
	if cstate != define.ContainerStateRunning {
		return define.HealthCheckContainerStopped, errors.Errorf("container %s is not running", c.ID())
	}
	if !c.HasHealthCheck() {
		return define.HealthCheckNotDefined, errors.Errorf("container %s has no defined healthcheck", c.ID())
	}
	return define.HealthCheckDefined, nil
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
	healthCheck, err := c.GetHealthCheckLog()
	if err != nil {
		return err
	}
	healthCheck.Status = status
	newResults, err := json.Marshal(healthCheck)
	if err != nil {
		return errors.Wrapf(err, "unable to marshall healthchecks for writing status")
	}
	return ioutil.WriteFile(c.healthCheckLogPath(), newResults, 0700)
}

// UpdateHealthCheckLog parses the health check results and writes the log
func (c *Container) updateHealthCheckLog(hcl define.HealthCheckLog, inStartPeriod bool) error {
	healthCheck, err := c.GetHealthCheckLog()
	if err != nil {
		return err
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
			healthCheck.FailingStreak += 1
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
		return errors.Wrapf(err, "unable to marshall healthchecks for writing")
	}
	return ioutil.WriteFile(c.healthCheckLogPath(), newResults, 0700)
}

// HealthCheckLogPath returns the path for where the health check log is
func (c *Container) healthCheckLogPath() string {
	return filepath.Join(filepath.Dir(c.state.RunDir), "healthcheck.log")
}

// GetHealthCheckLog returns HealthCheck results by reading the container's
// health check log file.  If the health check log file does not exist, then
// an empty healthcheck struct is returned
func (c *Container) GetHealthCheckLog() (define.HealthCheckResults, error) {
	var healthCheck define.HealthCheckResults
	if _, err := os.Stat(c.healthCheckLogPath()); os.IsNotExist(err) {
		return healthCheck, nil
	}
	b, err := ioutil.ReadFile(c.healthCheckLogPath())
	if err != nil {
		return healthCheck, errors.Wrap(err, "failed to read health check log file")
	}
	if err := json.Unmarshal(b, &healthCheck); err != nil {
		return healthCheck, errors.Wrapf(err, "failed to unmarshal existing healthcheck results in %s", c.healthCheckLogPath())
	}
	return healthCheck, nil
}

// HealthCheckStatus returns the current state of a container with a healthcheck
func (c *Container) HealthCheckStatus() (string, error) {
	if !c.HasHealthCheck() {
		return "", errors.Errorf("container %s has no defined healthcheck", c.ID())
	}
	results, err := c.GetHealthCheckLog()
	if err != nil {
		return "", errors.Wrapf(err, "unable to get healthcheck log for %s", c.ID())
	}
	return results.Status, nil
}

func (c *Container) disableHealthCheckSystemd() bool {
	if os.Getenv("DISABLE_HC_SYSTEMD") == "true" {
		return true
	}
	if c.config.HealthCheckConfig.Interval == 0 {
		return true
	}
	return false
}
