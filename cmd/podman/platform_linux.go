// +build linux

package main

import (
	"os"
	"path/filepath"

	"github.com/containers/libpod/pkg/rootless"
	"github.com/sirupsen/logrus"
)

// userRegistriesFile is the path to the per user registry configuration file.
var userRegistriesFile = filepath.Join(os.Getenv("HOME"), ".config/containers/registries.conf")

func CheckForRegistries() {
	if _, err := os.Stat("/etc/containers/registries.conf"); err != nil {
		if os.IsNotExist(err) {
			// If it is running in rootless mode, also check the user configuration file
			if rootless.IsRootless() {
				if _, err := os.Stat(userRegistriesFile); err != nil {
					logrus.Warnf("unable to find %s. some podman (image shortnames) commands may be limited", userRegistriesFile)
				}
				return
			}
			logrus.Warn("unable to find /etc/containers/registries.conf. some podman (image shortnames) commands may be limited")
		}
	}
}
