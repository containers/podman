package main

import (
	"os"
	"syscall"

	"github.com/containers/libpod/libpod"
	"github.com/docker/docker/pkg/signal"
	"github.com/sirupsen/logrus"
)

func ProxySignals(ctr *libpod.Container) {
	sigBuffer := make(chan os.Signal, 128)
	signal.CatchAll(sigBuffer)

	logrus.Debugf("Enabling signal proxying")

	go func() {
		for s := range sigBuffer {
			// Ignore SIGCHLD and SIGPIPE - these are mostly likely
			// intended for the podman command itself.
			if s == signal.SIGCHLD || s == signal.SIGPIPE {
				continue
			}

			if err := ctr.Kill(uint(s.(syscall.Signal))); err != nil {
				logrus.Errorf("Error forwarding signal %d to container %s: %v", s, ctr.ID(), err)
				signal.StopCatch(sigBuffer)
				syscall.Kill(syscall.Getpid(), s.(syscall.Signal))
			}
		}
	}()

	return
}
