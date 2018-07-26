// +build linux

package libpod

import (
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/sirupsen/logrus"
)

type containerPlatformState struct {

	// NetNSPath is the path of the container's network namespace
	// Will only be set if config.CreateNetNS is true, or the container was
	// told to join another container's network namespace
	NetNS ns.NetNS `json:"-"`
}

func (ctr *Container) setNamespace(netNSPath string, newState *containerState) error {
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
				logrus.Errorf("error joining network namespace for container %s", ctr.ID())
				ctr.valid = false
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

func (ctr *Container) setNamespaceStatePath() string {
	if ctr.state.NetNS != nil {
		return ctr.state.NetNS.Path()
	}
	return ""
}
