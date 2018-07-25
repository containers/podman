// +build !linux

package libpod

import (
	"github.com/sirupsen/logrus"
)

// parseNetNSBoltData sets ctr.state.NetNS, if any, from netNSBytes.
// Returns true if the data is valid.
func parseNetNSBoltData(ctr *Container, netNSBytes []byte) bool {
	if netNSBytes != nil {
		logrus.Errorf("error loading %s: network namespaces are not supported on this platform", ctr.ID())
		return false
	}
	return true
}
