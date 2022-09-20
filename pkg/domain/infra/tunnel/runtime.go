package tunnel

import (
	"context"
	"os"
	"syscall"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/signal"
	"github.com/sirupsen/logrus"
)

// Image-related runtime using an ssh-tunnel to utilize Podman service
type ImageEngine struct {
	ClientCtx context.Context
}

// Container-related runtime using an ssh-tunnel to utilize Podman service
type ContainerEngine struct {
	ClientCtx context.Context
}

// Container-related runtime using an ssh-tunnel to utilize Podman service
type SystemEngine struct {
	ClientCtx context.Context
}

func remoteProxySignals(ctrID string, killFunc func(string) error) {
	sigBuffer := make(chan os.Signal, signal.SignalBufferSize)
	signal.CatchAll(sigBuffer)

	logrus.Debugf("Enabling signal proxying")

	go func() {
		for s := range sigBuffer {
			syscallSignal := s.(syscall.Signal)
			if signal.IsSignalIgnoredBySigProxy(syscallSignal) {
				continue
			}
			signalName, err := signal.ParseSysSignalToName(syscallSignal)
			if err != nil {
				logrus.Infof("Ceasing signal %v forwarding to container %s as it has stopped: %s", s, ctrID, err)
			}
			if err := killFunc(signalName); err != nil {
				if err.Error() == define.ErrCtrStateInvalid.Error() {
					logrus.Debugf("Ceasing signal %q forwarding to container %s as it has stopped", signalName, ctrID)
				}
			}
		}
	}()
}
