package adapter

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/libpod/libpod"
	"github.com/docker/docker/pkg/term"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/client-go/tools/remotecommand"
)

// StartAttachCtr starts and (if required) attaches to a container
// if you change the signature of this function from os.File to io.Writer, it will trigger a downstream
// error. we may need to just lint disable this one.
func StartAttachCtr(ctx context.Context, ctr *libpod.Container, stdout, stderr, stdin *os.File, detachKeys string, sigProxy bool, startContainer bool, recursive bool) error { //nolint-interfacer
	resize := make(chan remotecommand.TerminalSize)

	haveTerminal := terminal.IsTerminal(int(os.Stdin.Fd()))

	// Check if we are attached to a terminal. If we are, generate resize
	// events, and set the terminal to raw mode
	if haveTerminal && ctr.Spec().Process.Terminal {
		logrus.Debugf("Handling terminal attach")

		subCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		resizeTty(subCtx, resize)

		oldTermState, err := term.SaveState(os.Stdin.Fd())
		if err != nil {
			return errors.Wrapf(err, "unable to save terminal state")
		}

		logrus.SetFormatter(&RawTtyFormatter{})
		if _, err := term.SetRawTerminal(os.Stdin.Fd()); err != nil {
			return err
		}

		defer func() {
			if err := restoreTerminal(oldTermState); err != nil {
				logrus.Errorf("unable to restore terminal: %q", err)
			}
		}()
	}

	streams := new(libpod.AttachStreams)
	streams.OutputStream = stdout
	streams.ErrorStream = stderr
	streams.InputStream = stdin
	streams.AttachOutput = true
	streams.AttachError = true
	streams.AttachInput = true

	if stdout == nil {
		logrus.Debugf("Not attaching to stdout")
		streams.AttachOutput = false
	}
	if stderr == nil {
		logrus.Debugf("Not attaching to stderr")
		streams.AttachError = false
	}
	if stdin == nil {
		logrus.Debugf("Not attaching to stdin")
		streams.AttachInput = false
	}

	if !startContainer {
		if sigProxy {
			ProxySignals(ctr)
		}

		return ctr.Attach(streams, detachKeys, resize)
	}

	attachChan, err := ctr.StartAndAttach(ctx, streams, detachKeys, resize, recursive)
	if err != nil {
		return err
	}

	if sigProxy {
		ProxySignals(ctr)
	}

	if stdout == nil && stderr == nil {
		fmt.Printf("%s\n", ctr.ID())
	}

	err = <-attachChan
	if err != nil {
		return errors.Wrapf(err, "error attaching to container %s", ctr.ID())
	}

	return nil
}
