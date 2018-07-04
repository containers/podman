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
			// Tear down the existing namespace
			if err := ctr.runtime.teardownNetNS(ctr); err != nil {
				logrus.Warnf(err.Error())
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
		// Tear down the old one
		if err := ctr.runtime.teardownNetNS(ctr); err != nil {
			logrus.Warnf(err.Error())
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
