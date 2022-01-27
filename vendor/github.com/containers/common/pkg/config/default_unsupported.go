// +build !linux,!windows

package config

// getDefaultMachineImage returns the default machine image stream
// On Linux/Mac, this returns the FCOS stream
func getDefaultMachineImage() string {
	return "testing"
}

// getDefaultMachineUser returns the user to use for rootless podman
func getDefaultMachineUser() string {
	return "core"
}

// isCgroup2UnifiedMode returns whether we are running in cgroup2 mode.
func isCgroup2UnifiedMode() (isUnified bool, isUnifiedErr error) {
	return false, nil
}

// getDefaultProcessLimits returns the nofile and nproc for the current process in ulimits format
func getDefaultProcessLimits() []string {
	return []string{}
}
