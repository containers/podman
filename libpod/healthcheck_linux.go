package libpod

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/coreos/go-systemd/dbus"
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

	var cmd = []string{"--unit", fmt.Sprintf("%s", c.ID()), fmt.Sprintf("--on-unit-inactive=%s", c.HealthCheckConfig().Interval.String()), "--timer-property=AccuracySec=1s", podman, "healthcheck", "run", c.ID()}

	conn, err := dbus.NewSystemdConnection()
	if err != nil {
		return errors.Wrapf(err, "unable to get systemd connection to add healthchecks")
	}
	conn.Close()
	logrus.Debugf("creating systemd-transient files: %s %s", "systemd-run", cmd)
	systemdRun := exec.Command("systemd-run", cmd...)
	_, err = systemdRun.CombinedOutput()
	if err != nil {
		return err
	}
	return nil
}

// startTimer starts a systemd timer for the healthchecks
func (c *Container) startTimer() error {
	if c.disableHealthCheckSystemd() {
		return nil
	}
	conn, err := dbus.NewSystemdConnection()
	if err != nil {
		return errors.Wrapf(err, "unable to get systemd connection to start healthchecks")
	}
	defer conn.Close()
	_, err = conn.StartUnit(fmt.Sprintf("%s.service", c.ID()), "fail", nil)
	return err
}

// removeTimer removes the systemd timer and unit files
// for the container
func (c *Container) removeTimer() error {
	if c.disableHealthCheckSystemd() {
		return nil
	}
	conn, err := dbus.NewSystemdConnection()
	if err != nil {
		return errors.Wrapf(err, "unable to get systemd connection to remove healthchecks")
	}
	defer conn.Close()
	serviceFile := fmt.Sprintf("%s.timer", c.ID())
	_, err = conn.StopUnit(serviceFile, "fail", nil)
	return err
}
