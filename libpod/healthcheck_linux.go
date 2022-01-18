package libpod

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/systemd"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// createTimer systemd timers for healthchecks of a container
func (c *Container) createTimer() error {
	if c.disableHealthCheckSystemd() {
		return nil
	}
	podman, err := os.Executable()
	if err != nil {
		return errors.Wrapf(err, "failed to get path for podman for a health check timer")
	}

	var cmd = []string{}
	if rootless.IsRootless() {
		cmd = append(cmd, "--user")
	}
	path := os.Getenv("PATH")
	if path != "" {
		cmd = append(cmd, "--setenv=PATH="+path)
	}
	cmd = append(cmd, "--unit", c.ID(), fmt.Sprintf("--on-unit-inactive=%s", c.HealthCheckConfig().Interval.String()), "--timer-property=AccuracySec=1s", podman, "healthcheck", "run", c.ID())

	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return errors.Wrapf(err, "unable to get systemd connection to add healthchecks")
	}
	conn.Close()
	logrus.Debugf("creating systemd-transient files: %s %s", "systemd-run", cmd)
	systemdRun := exec.Command("systemd-run", cmd...)
	if output, err := systemdRun.CombinedOutput(); err != nil {
		return errors.Errorf("%s", output)
	}
	return nil
}

// startTimer starts a systemd timer for the healthchecks
func (c *Container) startTimer() error {
	if c.disableHealthCheckSystemd() {
		return nil
	}
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return errors.Wrapf(err, "unable to get systemd connection to start healthchecks")
	}
	defer conn.Close()
	_, err = conn.StartUnit(fmt.Sprintf("%s.service", c.ID()), "fail", nil)
	return err
}

// removeTransientFiles removes the systemd timer and unit files
// for the container
func (c *Container) removeTransientFiles(ctx context.Context) error {
	if c.disableHealthCheckSystemd() {
		return nil
	}
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return errors.Wrapf(err, "unable to get systemd connection to remove healthchecks")
	}
	defer conn.Close()
	timerFile := fmt.Sprintf("%s.timer", c.ID())
	serviceFile := fmt.Sprintf("%s.service", c.ID())

	// If the service has failed (the healthcheck has failed), then
	// the .service file is not removed on stopping the unit file. If
	// we check the properties of the service, it will automatically
	// reset the state.  But checking the state takes msecs vs usecs to
	// blindly call reset.
	if err := conn.ResetFailedUnitContext(ctx, serviceFile); err != nil {
		logrus.Debugf("failed to reset unit file: %q", err)
	}

	// We want to ignore errors where the timer unit and/or service unit has already
	// been removed. The error return is generic so we have to check against the
	// string in the error
	if _, err = conn.StopUnitContext(ctx, serviceFile, "fail", nil); err != nil {
		if !strings.HasSuffix(err.Error(), ".service not loaded.") {
			return errors.Wrapf(err, "unable to remove service file")
		}
	}
	if _, err = conn.StopUnitContext(ctx, timerFile, "fail", nil); err != nil {
		if strings.HasSuffix(err.Error(), ".timer not loaded.") {
			return nil
		}
	}
	return err
}
