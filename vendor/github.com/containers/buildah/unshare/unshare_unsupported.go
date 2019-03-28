// +build !linux

package unshare

import (
	"os"
)

const (
	// UsernsEnvName is the environment variable, if set indicates in rootless mode
	UsernsEnvName = "_CONTAINERS_USERNS_CONFIGURED"
)

// IsRootless tells us if we are running in rootless mode
func IsRootless() bool {
	return false
}

// GetRootlessUID returns the UID of the user in the parent userNS
func GetRootlessUID() int {
	return os.Getuid()
}

// RootlessEnv returns the environment settings for the rootless containers
func RootlessEnv() []string {
	return append(os.Environ(), UsernsEnvName+"=")
}
