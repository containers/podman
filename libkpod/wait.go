package libkpod

import (
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

func isStopped(c *ContainerServer, ctr *oci.Container) bool {
	c.runtime.UpdateStatus(ctr)
	cStatus := c.runtime.ContainerStatus(ctr)
	if cStatus.Status == oci.ContainerStateStopped {
		return true
	}
	return false
}

// ContainerWait stops a running container with a grace period (i.e., timeout).
func (c *ContainerServer) ContainerWait(container string) (int32, error) {
	ctr, err := c.LookupContainer(container)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to find container %s", container)
	}

	err = wait.PollImmediateInfinite(1,
		func() (bool, error) {
			if !isStopped(c, ctr) {
				return false, nil
			} else { // nolint
				return true, nil // nolint
			} // nolint

		},
	)

	if err != nil {
		return 0, err
	}
	exitCode := ctr.State().ExitCode
	c.ContainerStateToDisk(ctr)
	return exitCode, nil
}
