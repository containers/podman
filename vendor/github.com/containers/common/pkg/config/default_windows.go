package config

import "os"

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

// getDefaultTmpDir for windows
func getDefaultTmpDir() string {
	// first check the Temp env var
	// https://answers.microsoft.com/en-us/windows/forum/all/where-is-the-temporary-folder/44a039a5-45ba-48dd-84db-fd700e54fd56
	if val, ok := os.LookupEnv("TEMP"); ok {
		return val
	}
	return os.Getenv("LOCALAPPDATA") + "\\Temp"
}

func getDefaultCgroupsMode() string {
	return "enabled"
}

func getDefaultLockType() string {
	return "shm"
}

func getLibpodTmpDir() string {
	return "/run/libpod"
}

// getDefaultMachineVolumes returns default mounted volumes (possibly with env vars, which will be expanded)
func getDefaultMachineVolumes() []string {
	return []string{}
}
