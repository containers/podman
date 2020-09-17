package terminal

import (
	"context"
	"os"
	"os/signal"

	lsignal "github.com/containers/podman/v2/pkg/signal"
	"github.com/moby/term"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/remotecommand"
)

// RawTtyFormatter ...
type RawTtyFormatter struct {
}

// getResize returns a TerminalSize command matching stdin's current
// size on success, and nil on errors.
func getResize() *remotecommand.TerminalSize {
	winsize, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		logrus.Warnf("Could not get terminal size %v", err)
		return nil
	}
	return &remotecommand.TerminalSize{
		Width:  winsize.Width,
		Height: winsize.Height,
	}
}

// Helper for prepareAttach - set up a goroutine to generate terminal resize events
func resizeTty(ctx context.Context, resize chan remotecommand.TerminalSize) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, lsignal.SIGWINCH)
	go func() {
		defer close(resize)
		// Update the terminal size immediately without waiting
		// for a SIGWINCH to get the correct initial size.
		resizeEvent := getResize()
		for {
			if resizeEvent == nil {
				select {
				case <-ctx.Done():
					return
				case <-sigchan:
					resizeEvent = getResize()
				}
			} else {
				select {
				case <-ctx.Done():
					return
				case <-sigchan:
					resizeEvent = getResize()
				case resize <- *resizeEvent:
					resizeEvent = nil
				}
			}
		}
	}()
}

func restoreTerminal(state *term.State) error {
	logrus.SetFormatter(&logrus.TextFormatter{})
	return term.RestoreTerminal(os.Stdin.Fd(), state)
}

// Format ...
func (f *RawTtyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	textFormatter := logrus.TextFormatter{}
	bytes, err := textFormatter.Format(entry)

	if err == nil {
		bytes = append(bytes, '\r')
	}

	return bytes, err
}

func handleTerminalAttach(ctx context.Context, resize chan remotecommand.TerminalSize) (context.CancelFunc, *term.State, error) {
	logrus.Debugf("Handling terminal attach")

	subCtx, cancel := context.WithCancel(ctx)

	resizeTty(subCtx, resize)

	oldTermState, err := term.SaveState(os.Stdin.Fd())
	if err != nil {
		// allow caller to not have to do any cleaning up if we error here
		cancel()
		return nil, nil, errors.Wrapf(err, "unable to save terminal state")
	}

	logrus.SetFormatter(&RawTtyFormatter{})
	if _, err := term.SetRawTerminal(os.Stdin.Fd()); err != nil {
		return cancel, nil, err
	}

	return cancel, oldTermState, nil
}
