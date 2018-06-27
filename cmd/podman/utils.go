package main

import (
	"fmt"
	"os"
	gosignal "os/signal"

	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/term"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/client-go/tools/remotecommand"
)

type RawTtyFormatter struct {
}

// Attach to a container
func attachCtr(ctr *libpod.Container, stdout, stderr, stdin *os.File, detachKeys string, sigProxy bool) error {
	resize := make(chan remotecommand.TerminalSize)

	haveTerminal := terminal.IsTerminal(int(os.Stdin.Fd()))

	// Check if we are attached to a terminal. If we are, generate resize
	// events, and set the terminal to raw mode
	if haveTerminal && ctr.Spec().Process.Terminal {
		logrus.Debugf("Handling terminal attach")

		resizeTerminate := make(chan interface{})
		defer func() {
			resizeTerminate <- true
			close(resizeTerminate)
		}()
		resizeTty(resize, resizeTerminate)

		oldTermState, err := term.SaveState(os.Stdin.Fd())
		if err != nil {
			return errors.Wrapf(err, "unable to save terminal state")
		}

		logrus.SetFormatter(&RawTtyFormatter{})
		term.SetRawTerminal(os.Stdin.Fd())

		defer restoreTerminal(oldTermState)
	}

	// There may be on the other size of the resize channel a goroutine trying to send.
	// So consume all data inside the channel before exiting to avoid a deadlock.
	defer func() {
		for {
			select {
			case <-resize:
				logrus.Debugf("Consumed resize command from channel")
			default:
				return
			}
		}
	}()

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

	if sigProxy {
		ProxySignals(ctr)
	}

	return ctr.Attach(streams, detachKeys, resize)
}

// Start and attach to a container
func startAttachCtr(ctr *libpod.Container, stdout, stderr, stdin *os.File, detachKeys string, sigProxy bool) error {
	resize := make(chan remotecommand.TerminalSize)

	haveTerminal := terminal.IsTerminal(int(os.Stdin.Fd()))

	// Check if we are attached to a terminal. If we are, generate resize
	// events, and set the terminal to raw mode
	if haveTerminal && ctr.Spec().Process.Terminal {
		logrus.Debugf("Handling terminal attach")

		resizeTerminate := make(chan interface{})
		defer func() {
			resizeTerminate <- true
			close(resizeTerminate)
		}()
		resizeTty(resize, resizeTerminate)

		oldTermState, err := term.SaveState(os.Stdin.Fd())
		if err != nil {
			return errors.Wrapf(err, "unable to save terminal state")
		}

		logrus.SetFormatter(&RawTtyFormatter{})
		term.SetRawTerminal(os.Stdin.Fd())

		defer restoreTerminal(oldTermState)
	}

	// There may be on the other size of the resize channel a goroutine trying to send.
	// So consume all data inside the channel before exiting to avoid a deadlock.
	defer func() {
		for {
			select {
			case <-resize:
				logrus.Debugf("Consumed resize command from channel")
			default:
				return
			}
		}
	}()

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

	attachChan, err := ctr.StartAndAttach(getContext(), streams, detachKeys, resize)
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

// Helper for prepareAttach - set up a goroutine to generate terminal resize events
func resizeTty(resize chan remotecommand.TerminalSize, resizeTerminate chan interface{}) {
	sigchan := make(chan os.Signal, 1)
	gosignal.Notify(sigchan, signal.SIGWINCH)
	sendUpdate := func() {
		winsize, err := term.GetWinsize(os.Stdin.Fd())
		if err != nil {
			logrus.Warnf("Could not get terminal size %v", err)
			return
		}
		resize <- remotecommand.TerminalSize{
			Width:  winsize.Width,
			Height: winsize.Height,
		}
	}
	go func() {
		defer close(resize)
		// Update the terminal size immediately without waiting
		// for a SIGWINCH to get the correct initial size.
		sendUpdate()
		for {
			select {
			case <-resizeTerminate:
				return

			case <-sigchan:
				sendUpdate()
			}
		}
	}()
}

func restoreTerminal(state *term.State) error {
	logrus.SetFormatter(&logrus.TextFormatter{})
	return term.RestoreTerminal(os.Stdin.Fd(), state)
}

func (f *RawTtyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	textFormatter := logrus.TextFormatter{}
	bytes, err := textFormatter.Format(entry)

	if err == nil {
		bytes = append(bytes, '\r')
	}

	return bytes, err
}
