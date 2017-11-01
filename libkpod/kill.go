package libkpod

import (
	"github.com/docker/docker/pkg/signal"
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/kubernetes-incubator/cri-o/utils"
	"github.com/pkg/errors"
	"os"
	"syscall"
)

// Reverse lookup signal string from its map
func findStringInSignalMap(killSignal syscall.Signal) (string, error) {
	for k, v := range signal.SignalMap {
		if v == killSignal {
			return k, nil
		}
	}
	return "", errors.Errorf("unable to convert signal to string")

}

// ContainerKill sends the user provided signal to the containers primary process.
func (c *ContainerServer) ContainerKill(container string, killSignal syscall.Signal) (string, error) { // nolint
	ctr, err := c.LookupContainer(container)
	if err != nil {
		return "", errors.Wrapf(err, "failed to find container %s", container)
	}
	c.runtime.UpdateStatus(ctr)
	cStatus := c.runtime.ContainerStatus(ctr)

	// If the container is not running, error and move on.
	if cStatus.Status != oci.ContainerStateRunning {
		return "", errors.Errorf("cannot kill container %s: it is not running", container)
	}
	signalString, err := findStringInSignalMap(killSignal)
	if err != nil {
		return "", err
	}
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, c.runtime.Path(ctr), "kill", ctr.ID(), signalString); err != nil {
		return "", err
	}
	c.ContainerStateToDisk(ctr)
	return ctr.ID(), nil
}
