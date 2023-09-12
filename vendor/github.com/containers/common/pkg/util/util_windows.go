//go:build windows
// +build windows

package util

import (
	"errors"

	terminal "golang.org/x/term"
)

// getRuntimeDir returns the runtime directory
func GetRuntimeDir() (string, error) {
	return "", errors.New("this function is not implemented for windows")
}

// ReadPassword reads a password from the terminal.
func ReadPassword(fd int) ([]byte, error) {
	oldState, err := terminal.GetState(fd)
	if err != nil {
		return make([]byte, 0), err
	}
	buf, err := terminal.ReadPassword(fd)
	if oldState != nil {
		_ = terminal.Restore(fd, oldState)
	}
	return buf, err
}
