package main

import (
	"fmt"
	"os"
	gosignal "os/signal"

	"github.com/containers/storage"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/term"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/client-go/tools/remotecommand"
)

// Generate a new libpod runtime configured by command line options
func getRuntime(c *cli.Context) (*libpod.Runtime, error) {
	options := []libpod.RuntimeOption{}

	if c.GlobalIsSet("root") || c.GlobalIsSet("runroot") ||
		c.GlobalIsSet("storage-opt") || c.GlobalIsSet("storage-driver") {
		storageOpts := storage.DefaultStoreOptions

		if c.GlobalIsSet("root") {
			storageOpts.GraphRoot = c.GlobalString("root")
		}
		if c.GlobalIsSet("runroot") {
			storageOpts.RunRoot = c.GlobalString("runroot")
		}
		if c.GlobalIsSet("storage-driver") {
			storageOpts.GraphDriverName = c.GlobalString("storage-driver")
		}
		if c.GlobalIsSet("storage-opt") {
			storageOpts.GraphDriverOptions = c.GlobalStringSlice("storage-opt")
		}

		options = append(options, libpod.WithStorageConfig(storageOpts))
	}

	// TODO CLI flags for image config?
	// TODO CLI flag for signature policy?

	if c.GlobalIsSet("runtime") {
		options = append(options, libpod.WithOCIRuntime(c.GlobalString("runtime")))
	}

	if c.GlobalIsSet("conmon") {
		options = append(options, libpod.WithConmonPath(c.GlobalString("conmon")))
	}

	// TODO flag to set CGroup manager?
	// TODO flag to set libpod static dir?
	// TODO flag to set libpod tmp dir?

	if c.GlobalIsSet("cni-config-dir") {
		options = append(options, libpod.WithCNIConfigDir(c.GlobalString("cni-config-dir")))
	}
	if c.GlobalIsSet("default-mounts-file") {
		options = append(options, libpod.WithDefaultMountsFile(c.GlobalString("default-mounts-file")))
	}
	options = append(options, libpod.WithHooksDir(c.GlobalString("hooks-dir-path")))

	// TODO flag to set CNI plugins dir?

	return libpod.NewRuntime(options...)
}

// Attach to a container
func attachCtr(ctr *libpod.Container, stdout, stderr, stdin *os.File, detachKeys string, sigProxy bool) error {
	resize := make(chan remotecommand.TerminalSize)
	defer close(resize)

	haveTerminal := terminal.IsTerminal(int(os.Stdin.Fd()))

	// Check if we are attached to a terminal. If we are, generate resize
	// events, and set the terminal to raw mode
	if haveTerminal && ctr.Spec().Process.Terminal {
		logrus.Debugf("Handling terminal attach")

		resizeTty(resize)

		oldTermState, err := term.SaveState(os.Stdin.Fd())
		if err != nil {
			return errors.Wrapf(err, "unable to save terminal state")
		}

		term.SetRawTerminal(os.Stdin.Fd())

		defer term.RestoreTerminal(os.Stdin.Fd(), oldTermState)
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

	if sigProxy {
		ProxySignals(ctr)
	}

	return ctr.Attach(streams, detachKeys, resize)
}

// Start and attach to a container
func startAttachCtr(ctr *libpod.Container, stdout, stderr, stdin *os.File, detachKeys string, sigProxy bool) error {
	resize := make(chan remotecommand.TerminalSize)
	defer close(resize)

	haveTerminal := terminal.IsTerminal(int(os.Stdin.Fd()))

	// Check if we are attached to a terminal. If we are, generate resize
	// events, and set the terminal to raw mode
	if haveTerminal && ctr.Spec().Process.Terminal {
		logrus.Debugf("Handling terminal attach")

		resizeTty(resize)

		oldTermState, err := term.SaveState(os.Stdin.Fd())
		if err != nil {
			return errors.Wrapf(err, "unable to save terminal state")
		}

		term.SetRawTerminal(os.Stdin.Fd())

		defer term.RestoreTerminal(os.Stdin.Fd(), oldTermState)
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
func resizeTty(resize chan remotecommand.TerminalSize) {
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
		for range sigchan {
			sendUpdate()
		}
	}()
}
