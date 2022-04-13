package homedir

// Copyright 2013-2018 Docker, Inc.
// NOTE: this package has originally been copied from github.com/docker/docker.

import (
	"os"
)

// Key returns the env var name for the user's home dir based on
// the platform being run on
func Key() string {
	return "USERPROFILE"
}

// Get returns the home directory of the current user with the help of
// environment variables depending on the target operating system.
// Returned path should be used with "path/filepath" to form new paths.
func Get() string {
	home := os.Getenv(Key())
	if home != "" {
		return home
	}
	home, _ = os.UserHomeDir()
	return home
}

// GetShortcutString returns the string that is shortcut to user's home directory
// in the native shell of the platform running on.
func GetShortcutString() string {
	return "%USERPROFILE%" // be careful while using in format functions
}
