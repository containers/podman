// +build !linux

package libpod

// replaceNetNS is exclusive to the Linux platform and is a no-op elsewhere
func replaceNetNS(netNSPath string, ctr *Container, newState *containerState) error {
	return nil
}

// getNetNSPath is exclusive to the Linux platform and is a no-op elsewhere
func getNetNSPath(ctr *Container) string {
	return ""
}
