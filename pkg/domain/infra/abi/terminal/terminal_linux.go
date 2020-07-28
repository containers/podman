package terminal

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/client-go/tools/remotecommand"
)

// ExecAttachCtr execs and attaches to a container
func ExecAttachCtr(ctx context.Context, ctr *libpod.Container, execConfig *libpod.ExecConfig, streams *define.AttachStreams) (int, error) {
	resize := make(chan remotecommand.TerminalSize)
	haveTerminal := terminal.IsTerminal(int(os.Stdin.Fd()))

	// Check if we are attached to a terminal. If we are, generate resize
	// events, and set the terminal to raw mode
	if haveTerminal && execConfig.Terminal {
		cancel, oldTermState, err := handleTerminalAttach(ctx, resize)
		if err != nil {
			return -1, err
		}
		defer cancel()
		defer func() {
			if err := restoreTerminal(oldTermState); err != nil {
				logrus.Errorf("unable to restore terminal: %q", err)
			}
		}()
	}

	return ctr.Exec(execConfig, streams, resize)
}

// StartAttachCtr starts and (if required) attaches to a container
// if you change the signature of this function from os.File to io.Writer, it will trigger a downstream
// error. we may need to just lint disable this one.
func StartAttachCtr(ctx context.Context, ctr *libpod.Container, stdout, stderr, stdin *os.File, detachKeys string, sigProxy bool, startContainer bool, recursive bool) error { //nolint-interfacer
	resize := make(chan remotecommand.TerminalSize)

	haveTerminal := terminal.IsTerminal(int(os.Stdin.Fd()))

	// Check if we are attached to a terminal. If we are, generate resize
	// events, and set the terminal to raw mode
	if haveTerminal && ctr.Spec().Process.Terminal {
		cancel, oldTermState, err := handleTerminalAttach(ctx, resize)
		if err != nil {
			return err
		}
		defer func() {
			if err := restoreTerminal(oldTermState); err != nil {
				logrus.Errorf("unable to restore terminal: %q", err)
			}
		}()
		defer cancel()
	}

	streams := new(define.AttachStreams)
	streams.OutputStream = stdout
	streams.ErrorStream = stderr
	streams.InputStream = bufio.NewReader(stdin)
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
