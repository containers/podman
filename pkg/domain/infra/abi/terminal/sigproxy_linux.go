package terminal

import (
	"os"
	"syscall"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/shutdown"
	"github.com/containers/podman/v4/pkg/signal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Make sure the signal buffer is sufficiently big.
// runc is using the same value.
const signalBufferSize = 2048

// ProxySignals ...
func ProxySignals(ctr *libpod.Container) {
	// Stop catching the shutdown signals (SIGINT, SIGTERM) - they're going
	// to the container now.
	shutdown.Stop() // nolint: errcheck

	sigBuffer := make(chan os.Signal, signalBufferSize)
	signal.CatchAll(sigBuffer)

	logrus.Debugf("Enabling signal proxying")

	go func() {
		for s := range sigBuffer {
			// Ignore SIGCHLD and SIGPIPE - these are mostly likely
			// intended for the podman command itself.
			// SIGURG was added because of golang 1.14 and its preemptive changes
			// causing more signals to "show up".
			// https://github.com/containers/podman/issues/5483
			if s == syscall.SIGCHLD || s == syscall.SIGPIPE || s == syscall.SIGURG {
				continue
			}

			if err := ctr.Kill(uint(s.(syscall.Signal))); err != nil {
				if errors.Cause(err) == define.ErrCtrStateInvalid {
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
