//go:build (linux || freebsd) && !remote

package terminal

import (
	"errors"
	"os"
	"syscall"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/libpod/shutdown"
	"github.com/containers/podman/v5/pkg/signal"
	"github.com/sirupsen/logrus"
)

// ProxySignals ...
func ProxySignals(ctr *libpod.Container) {
	// Stop catching the shutdown signals (SIGINT, SIGTERM) - they're going
	// to the container now.
	shutdown.Stop() //nolint: errcheck

	sigBuffer := make(chan os.Signal, signal.SignalBufferSize)
	signal.CatchAll(sigBuffer)

	logrus.Debugf("Enabling signal proxying")

	go func() {
		for s := range sigBuffer {
			syscallSignal := s.(syscall.Signal)
			if signal.IsSignalIgnoredBySigProxy(syscallSignal) {
				continue
			}

			if err := ctr.Kill(uint(syscallSignal)); err != nil {
				if errors.Is(err, define.ErrCtrStateInvalid) {
					logrus.Infof("Ceasing signal forwarding to container %s as it has stopped", ctr.ID())
				} else {
					logrus.Errorf("forwarding signal %d to container %s: %v", s, ctr.ID(), err)
				}
				// If the container dies, and we find out here,
				// we need to forward that one signal to
				// ourselves so that it is not lost, and then
				// we terminate the proxy and let the defaults
				// play out.
				signal.StopCatch(sigBuffer)
				if err := syscall.Kill(syscall.Getpid(), s.(syscall.Signal)); err != nil {
					logrus.Errorf("Failed to kill pid %d", syscall.Getpid())
				}
				return
			}
		}
	}()
}
