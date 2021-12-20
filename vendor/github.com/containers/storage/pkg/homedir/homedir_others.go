// +build !linux,!darwin,!freebsd

package homedir

// Copyright 2013-2018 Docker, Inc.
// NOTE: this package has originally been copied from github.com/docker/docker.

import (
	"errors"
)

// GetRuntimeDir is unsupported on non-linux system.
func GetRuntimeDir() (string, error) {
	return "", errors.New("homedir.GetRuntimeDir() is not supported on this system")
}

// StickRuntimeDirContents is unsupported on non-linux system.
func StickRuntimeDirContents(files []string) ([]string, error) {
	return nil, errors.New("homedir.StickRuntimeDirContents() is not supported on this system")
}
