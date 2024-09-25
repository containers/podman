package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	oldMaxSize = uint64(1048576)
)

func getDefaultCgroupsMode() string {
	return "enabled"
}

// getDefaultProcessLimits returns the nproc for the current process in ulimits format
// Note that nfile sometimes cannot be set to unlimited, and the limit is hardcoded
// to (oldMaxSize) 1048576 (2^20), see: http://stackoverflow.com/a/1213069/1811501
// In rootless containers this will fail, and the process will just use its current limits
func getDefaultProcessLimits() []string {
	rlim := unix.Rlimit{Cur: oldMaxSize, Max: oldMaxSize}
	oldrlim := rlim
	// Attempt to set file limit and process limit to pid_max in OS
	dat, err := os.ReadFile("/proc/sys/kernel/pid_max")
	if err == nil {
		val := strings.TrimSuffix(string(dat), "\n")
		max, err := strconv.ParseUint(val, 10, 64)
		if err == nil {
			rlim = unix.Rlimit{Cur: max, Max: max}
		}
	}
	defaultLimits := []string{}
	if err := unix.Setrlimit(unix.RLIMIT_NPROC, &rlim); err == nil {
		defaultLimits = append(defaultLimits, fmt.Sprintf("nproc=%d:%d", rlim.Cur, rlim.Max))
	} else if err := unix.Setrlimit(unix.RLIMIT_NPROC, &oldrlim); err == nil {
		defaultLimits = append(defaultLimits, fmt.Sprintf("nproc=%d:%d", oldrlim.Cur, oldrlim.Max))
	}
	return defaultLimits
}

// getDefaultTmpDir for linux
func getDefaultTmpDir() string {
	// first check the TMPDIR env var
	if path, found := os.LookupEnv("TMPDIR"); found {
		return path
	}
	return "/var/tmp"
}

func getDefaultLockType() string {
	return "shm"
}

func getLibpodTmpDir() string {
	return "/run/libpod"
}

// getDefaultMachineVolumes returns default mounted volumes (possibly with env vars, which will be expanded)
func getDefaultMachineVolumes() []string {
	return []string{"$HOME:$HOME"}
}

func getDefaultComposeProviders() []string {
	return defaultUnixComposeProviders
}
