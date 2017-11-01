// +build linux

package main

import (
	systemdDaemon "github.com/coreos/go-systemd/daemon"
	"github.com/sirupsen/logrus"
)

func sdNotify() {
	if _, err := systemdDaemon.SdNotify(true, "READY=1"); err != nil {
		logrus.Warnf("Failed to sd_notify systemd: %v", err)
	}
}

// notifySystem sends a message to the host when the server is ready to be used
func notifySystem() {
	// Tell the init daemon we are accepting requests
	go sdNotify()
}
