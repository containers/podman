//go:build windows
// +build windows

package util

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/containers/storage/pkg/homedir"
	"golang.org/x/sys/windows"
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

// GetRootlessPauseProcessPidPath returns the path to the file that holds the pid for
// the pause process
func GetRootlessPauseProcessPidPath() (string, error) {
	return "", fmt.Errorf("GetRootlessPauseProcessPidPath: %w", errNotImplemented)
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

// Wait until the given PID exits. Returns nil if wait was successful, errors on
// unexpected condition (IE, pid was not valid)
func WaitForPIDExit(pid uint) error {
	const PROCESS_ALL_ACCESS = 0x1F0FFF

	// We need to turn the PID into a Windows handle.
	// To do this we need Windows' OpenProcess func.
	// To get that, we need to open the kernel32 DLL.
	kernel32, err := windows.LoadDLL("kernel32.dll")
	if err != nil {
		return fmt.Errorf("loading kernel32 dll: %w", err)
	}

	openProc, err := kernel32.FindProc("OpenProcess")
	if err != nil {
		return fmt.Errorf("loading OpenProcess API: %w", err)
	}

	handle, _, err := openProc.Call(uintptr(PROCESS_ALL_ACCESS), uintptr(1), uintptr(pid))
	if err != nil {
		return fmt.Errorf("converting PID to handle: %w", err)
	}

	// We can now wait for the handle.
	_, err = windows.WaitForSingleObject(windows.Handle(handle), 0)
	if err != nil {
		return fmt.Errorf("waiting for handle: %w", err)
	}

	return nil
}
