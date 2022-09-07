//go:build freebsd
// +build freebsd

package libpod

// replaceNetNS handle network namespace transitions after updating a
// container's state.
func replaceNetNS(netNSPath string, ctr *Container, newState *ContainerState) error {
	if netNSPath != "" {
		// On FreeBSD, we just record the network jail's name in our state.
		newState.NetNS = &jailNetNS{Name: netNSPath}
	} else {
		newState.NetNS = nil
	}
	return nil
}

// getNetNSPath retrieves the netns path to be stored in the database
func getNetNSPath(ctr *Container) string {
	if ctr.state.NetNS != nil {
		return ctr.state.NetNS.Name
	} else {
		return ""
	}
}
