//go:build windows
// +build windows

package util

import (
	"path/filepath"

	"github.com/containers/storage/pkg/homedir"
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

// GetRootlessPauseProcessPidPath returns the path to the file that holds the pid for
// the pause process
func GetRootlessPauseProcessPidPathGivenDir(unused string) (string, error) {
	return "", errors.Wrap(errNotImplemented, "GetRootlessPauseProcessPidPath")
}

// GetRuntimeDir returns the runtime directory
func GetRuntimeDir() (string, error) {
	data, err := homedir.GetDataHome()
	if err != nil {
		return "", err
	}
	runtimeDir := filepath.Join(data, "containers", "podman")
	return runtimeDir, nil
}

// GetRootlessConfigHomeDir returns the config home directory when running as non root
func GetRootlessConfigHomeDir() (string, error) {
	return "", errors.New("this function is not implemented for windows")
}
