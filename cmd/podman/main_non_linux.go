//go:build windows || darwin || freebsd

package main

import (
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/cgroups"
)

func checkSupportedCgroups() {
	unified, _ := cgroups.IsCgroup2UnifiedMode()
	if !unified {
		logrus.Debugln("Non-linux environment. Non-fatal cgroups check")
	}
}
