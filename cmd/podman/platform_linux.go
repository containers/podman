// +build linux

package main

import (
	"os"

	"github.com/sirupsen/logrus"
)

func CheckForRegistries() {
	if _, err := os.Stat("/etc/containers/registries.conf"); err != nil {
		if os.IsNotExist(err) {
			logrus.Warn("unable to find /etc/containers/registries.conf. some podman (image shortnames) commands may be limited")
		}
	}
}
