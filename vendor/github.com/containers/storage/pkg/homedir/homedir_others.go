//go:build !linux && !darwin && !freebsd && !windows
// +build !linux,!darwin,!freebsd,!windows

package homedir

// Copyright 2013-2018 Docker, Inc.
// NOTE: this package has originally been copied from github.com/docker/docker.

import (
	"errors"
	"os"
	"path/filepath"
)

// GetRuntimeDir is unsupported on non-linux system.
func GetRuntimeDir() (string, error) {
	return "", errors.New("homedir.GetRuntimeDir() is not supported on this system")
}

// StickRuntimeDirContents is unsupported on non-linux system.
func StickRuntimeDirContents(files []string) ([]string, error) {
	return nil, errors.New("homedir.StickRuntimeDirContents() is not supported on this system")
}

// GetConfigHome returns XDG_CONFIG_HOME.
// GetConfigHome returns $HOME/.config and nil error if XDG_CONFIG_HOME is not set.
//
// See also https://standards.freedesktop.org/basedir-spec/latest/ar01s03.html
func GetConfigHome() (string, error) {
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return xdgConfigHome, nil
	}
	home := Get()
	if home == "" {
		return "", errors.New("could not get either XDG_CONFIG_HOME or HOME")
	}
	return filepath.Join(home, ".config"), nil
}
