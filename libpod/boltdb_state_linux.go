// +build linux

package libpod

import (
	"github.com/containers/podman/v2/libpod/define"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// replaceNetNS handle network namespace transitions after updating a
// container's state.
func replaceNetNS(netNSPath string, ctr *Container, newState *ContainerState) error {
	if netNSPath != "" {
		// Check if the container's old state has a good netns
		if ctr.state.NetNS != nil && netNSPath == ctr.state.NetNS.Path() {
			newState.NetNS = ctr.state.NetNS
		} else {
			// Close the existing namespace.
			// Whoever removed it from the database already tore it down.
			if err := ctr.runtime.closeNetNS(ctr); err != nil {
				return err
			}

			// Open the new network namespace
			ns, err := joinNetNS(netNSPath)
			if err == nil {
				newState.NetNS = ns
			} else {
				if ctr.ensureState(define.ContainerStateRunning, define.ContainerStatePaused) {
					return errors.Wrapf(err, "error joning network namespace of container %s", ctr.ID())
				}

				logrus.Errorf("error joining network namespace for container %s: %v", ctr.ID(), err)
				ctr.state.NetNS = nil
			}
		}
	} else {
		// The container no longer has a network namespace
		// Close the old one, whoever removed it from the DB should have
		// cleaned it up already.
		if err := ctr.runtime.closeNetNS(ctr); err != nil {
			return err
		}
	}
	return nil
}

// getNetNSPath retrieves the netns path to be stored in the database
func getNetNSPath(ctr *Container) string {
	if ctr.state.NetNS != nil {
		return ctr.state.NetNS.Path()
	}
	return ""
}
