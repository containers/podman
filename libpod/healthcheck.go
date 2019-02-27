package libpod

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// HealthCheckStatus represents the current state of a container
type HealthCheckStatus int

const (
	// HealthCheckSuccess means the health worked
	HealthCheckSuccess HealthCheckStatus = iota
	// HealthCheckFailure means the health ran and failed
	HealthCheckFailure HealthCheckStatus = iota
	// HealthCheckContainerStopped means the health check cannot
	// be run because the container is stopped
	HealthCheckContainerStopped HealthCheckStatus = iota
	// HealthCheckContainerNotFound means the container could
	// not be found in local store
	HealthCheckContainerNotFound HealthCheckStatus = iota
	// HealthCheckNotDefined means the container has no health
	// check defined in it
	HealthCheckNotDefined HealthCheckStatus = iota
	// HealthCheckInternalError means somes something failed obtaining or running
	// a given health check
	HealthCheckInternalError HealthCheckStatus = iota
	// HealthCheckDefined means the healthcheck was found on the container
	HealthCheckDefined HealthCheckStatus = iota
)

// HealthCheck verifies the state and validity of the healthcheck configuration
// on the container and then executes the healthcheck
func (r *Runtime) HealthCheck(name string) (HealthCheckStatus, error) {
	container, err := r.LookupContainer(name)
	if err != nil {
		return HealthCheckContainerNotFound, errors.Wrapf(err, "unable to lookup %s to perform a health check", name)
	}
	hcStatus, err := checkHealthCheckCanBeRun(container)
	if err == nil {
		return container.RunHealthCheck()
	}
	return hcStatus, err
}

// RunHealthCheck runs the health check as defined by the container
func (c *Container) RunHealthCheck() (HealthCheckStatus, error) {
	var newCommand []string
	hcStatus, err := checkHealthCheckCanBeRun(c)
	if err != nil {
		return hcStatus, err
	}
	hcCommand := c.HealthCheckConfig().Test
	if len(hcCommand) > 0 && hcCommand[0] == "CMD-SHELL" {
		newCommand = []string{"sh", "-c"}
		newCommand = append(newCommand, hcCommand[1:]...)
	} else {
		newCommand = hcCommand
	}
	// TODO when history/logging is implemented for healthcheck, we need to change the output streams
	// so we can capture i/o
	streams := new(AttachStreams)
	streams.OutputStream = os.Stdout
	streams.ErrorStream = os.Stderr
	streams.InputStream = os.Stdin
	streams.AttachOutput = true
	streams.AttachError = true
	streams.AttachInput = true

	logrus.Debugf("executing health check command %s for %s", strings.Join(newCommand, " "), c.ID())
	if err := c.Exec(false, false, []string{}, newCommand, "", "", streams, 0); err != nil {
		return HealthCheckFailure, err
	}
	return HealthCheckSuccess, nil
}

func checkHealthCheckCanBeRun(c *Container) (HealthCheckStatus, error) {
	cstate, err := c.State()
	if err != nil {
		return HealthCheckInternalError, err
	}
	if cstate != ContainerStateRunning {
		return HealthCheckContainerStopped, errors.Errorf("container %s is not running", c.ID())
	}
	if !c.HasHealthCheck() {
		return HealthCheckNotDefined, errors.Errorf("container %s has no defined healthcheck", c.ID())
	}
	return HealthCheckDefined, nil
}
