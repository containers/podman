//go:build freebsd
// +build freebsd

package libpod

// replaceNetNS handle network namespace transitions after updating a
// container's state.
func replaceNetNS(netNSPath string, ctr *Container, newState *ContainerState) error {
	// On FreeBSD, we just record the network jail's name in our state.
	newState.NetworkJail = netNSPath
	return nil
}

// getNetNSPath retrieves the netns path to be stored in the database
func getNetNSPath(ctr *Container) string {
	return ctr.state.NetworkJail
}
