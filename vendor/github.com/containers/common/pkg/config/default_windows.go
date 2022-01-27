package config

// getDefaultImage returns the default machine image stream
// On Windows this refers to the Fedora major release number
func getDefaultMachineImage() string {
	return "35"
}

// getDefaultMachineUser returns the user to use for rootless podman
func getDefaultMachineUser() string {
	return "user"
}

// isCgroup2UnifiedMode returns whether we are running in cgroup2 mode.
func isCgroup2UnifiedMode() (isUnified bool, isUnifiedErr error) {
	return false, nil
}

// getDefaultProcessLimits returns the nofile and nproc for the current process in ulimits format
func getDefaultProcessLimits() []string {
	return []string{}
}
