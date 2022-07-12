package terminal

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/containers/common/pkg/resize"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
)

// ExecAttachCtr execs and attaches to a container
func ExecAttachCtr(ctx context.Context, ctr *libpod.Container, execConfig *libpod.ExecConfig, streams *define.AttachStreams) (int, error) {
	var resizechan chan resize.TerminalSize
	haveTerminal := term.IsTerminal(int(os.Stdin.Fd()))

	// Check if we are attached to a terminal. If we are, generate resize
	// events, and set the terminal to raw mode
	if haveTerminal && execConfig.Terminal {
		resizechan = make(chan resize.TerminalSize)
		cancel, oldTermState, err := handleTerminalAttach(ctx, resizechan)
		if err != nil {
			return -1, err
		}
		defer cancel()
		defer func() {
			if err := restoreTerminal(oldTermState); err != nil {
				logrus.Errorf("Unable to restore terminal: %q", err)
			}
		}()
	}
	return ctr.Exec(execConfig, streams, resizechan)
}

// StartAttachCtr starts and (if required) attaches to a container
// if you change the signature of this function from os.File to io.Writer, it will trigger a downstream
// error. we may need to just lint disable this one.
func StartAttachCtr(ctx context.Context, ctr *libpod.Container, stdout, stderr, stdin *os.File, detachKeys string, sigProxy bool, startContainer bool) error { //nolint: interfacer
	resize := make(chan resize.TerminalSize)

	haveTerminal := term.IsTerminal(int(os.Stdin.Fd()))

	// Check if we are attached to a terminal. If we are, generate resize
	// events, and set the terminal to raw mode
	if haveTerminal && ctr.Spec().Process.Terminal {
		cancel, oldTermState, err := handleTerminalAttach(ctx, resize)
		if err != nil {
			return err
		}
		defer func() {
			if err := restoreTerminal(oldTermState); err != nil {
				logrus.Errorf("Unable to restore terminal: %q", err)
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

	attachChan, err := ctr.StartAndAttach(ctx, streams, detachKeys, resize, true)
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
		return fmt.Errorf("error attaching to container %s: %w", ctr.ID(), err)
	}

	return nil
}
