//go:build windows

package util

import (
	"errors"
	"fmt"
	"path/filepath"

	"go.podman.io/podman/v6/pkg/machine/env"
	"go.podman.io/storage/pkg/homedir"
)

var errNotImplemented = errors.New("not yet implemented")

// IsCgroup2UnifiedMode returns whether we are running in cgroup 2 unified mode.
func IsCgroup2UnifiedMode() (bool, error) {
	return false, fmt.Errorf("IsCgroup2Unified: %w", errNotImplemented)
}

// GetContainerPidInformationDescriptors returns a string slice of all supported
// format descriptors of GetContainerPidInformation.
func GetContainerPidInformationDescriptors() ([]string, error) {
	return nil, fmt.Errorf("GetContainerPidInformationDescriptors: %w", errNotImplemented)
}

// GetRootlessStateDir returns the directory that holds the rootless state
// (pause.pid and ns_handles files).
func GetRootlessStateDir() (string, error) {
	return "", fmt.Errorf("GetRootlessStateDir: %w", errNotImplemented)
}

// GetRootlessRuntimeDir returns the runtime directory
func GetRootlessRuntimeDir() (string, error) {
	data, err := homedir.GetDataHome()
	if err != nil {
		return "", err
	}
	rtDir := env.GetRuntimeDirSuffix()
	runtimeDir := filepath.Join(data, "containers", rtDir)
	return runtimeDir, nil
}

// GetRootlessConfigHomeDir returns the config home directory when running as non root
func GetRootlessConfigHomeDir() (string, error) {
	return "", errors.New("this function is not implemented for windows")
}
