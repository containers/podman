// +build windows

package util

import (
	"github.com/pkg/errors"
)

var errNotImplemented = errors.New("not yet implemented")

// IsCgroup2UnifiedMode returns whether we are running in cgroup 2 unified mode.
func IsCgroup2UnifiedMode() (bool, error) {
	return false, errors.Wrap(errNotImplemented, "IsCgroup2Unified")
}

// GetContainerPidInformationDescriptors returns a string slice of all supported
// format descriptors of GetContainerPidInformation.
func GetContainerPidInformationDescriptors() ([]string, error) {
	return nil, errors.Wrap(errNotImplemented, "GetContainerPidInformationDescriptors")
}

// GetRootlessPauseProcessPidPath returns the path to the file that holds the pid for
// the pause process
func GetRootlessPauseProcessPidPath() (string, error) {
	return "", errors.Wrap(errNotImplemented, "GetRootlessPauseProcessPidPath")
}

// GetRootlessRuntimeDir returns the runtime directory when running as non root
func GetRootlessRuntimeDir() (string, error) {
	return "", errors.Wrap(errNotImplemented, "GetRootlessRuntimeDir")
}

// GetRootlessConfigHomeDir returns the config home directory when running as non root
func GetRootlessConfigHomeDir() (string, error) {
	return "", errors.Wrap(errNotImplemented, "GetRootlessConfigHomeDir")
}
