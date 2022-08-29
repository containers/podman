//go:build !linux && !freebsd
// +build !linux,!freebsd

package libpod

import (
	"errors"
)

// replaceNetNS handle network namespace transitions after updating a
// container's state.
func replaceNetNS(netNSPath string, ctr *Container, newState *ContainerState) error {
	return errors.New("replaceNetNS not supported on this platform")
}

// getNetNSPath retrieves the netns path to be stored in the database
func getNetNSPath(ctr *Container) string {
	return ""
}
