// +build linux

package libpod

import (
	"github.com/sirupsen/logrus"
)

// parseNetNSBoltData sets ctr.state.NetNS, if any, from netNSBytes.
// Returns true if the data is valid.
func parseNetNSBoltData(ctr *Container, netNSBytes []byte) bool {
	// The container may not have a network namespace, so it's OK if this is
	// nil
	if netNSBytes != nil {
		nsPath := string(netNSBytes)
		netNS, err := joinNetNS(nsPath)
		if err == nil {
			ctr.state.NetNS = netNS
		} else {
			logrus.Errorf("error joining network namespace for container %s", ctr.ID())
			return false
		}
	}
	return true
}
