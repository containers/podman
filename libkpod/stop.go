package libkpod

import (
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// ContainerStop stops a running container with a grace period (i.e., timeout).
func (c *ContainerServer) ContainerStop(ctx context.Context, container string, timeout int64) (string, error) {
	ctr, err := c.LookupContainer(container)
	if err != nil {
		return "", errors.Wrapf(err, "failed to find container %s", container)
	}
	ctrID := ctr.ID()

	cStatus := c.runtime.ContainerStatus(ctr)
	switch cStatus.Status {

	case oci.ContainerStatePaused:
		return "", errors.Errorf("cannot stop paused container %s", ctrID)
	default:
		if cStatus.Status != oci.ContainerStateStopped {
			if err := c.runtime.StopContainer(ctx, ctr, timeout); err != nil {
				return "", errors.Wrapf(err, "failed to stop container %s", ctrID)
			}
			if err := c.storageRuntimeServer.StopContainer(ctrID); err != nil {
				return "", errors.Wrapf(err, "failed to unmount container %s", ctrID)
			}
		}
	}

	c.ContainerStateToDisk(ctr)

	return ctrID, nil
}
