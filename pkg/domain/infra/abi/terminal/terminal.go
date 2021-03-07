package terminal

import (
	"context"
	"os"
	"os/signal"

	"github.com/containers/podman/v3/libpod/define"
	lsignal "github.com/containers/podman/v3/pkg/signal"
	"github.com/moby/term"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// getResize returns a TerminalSize command matching stdin's current
// size on success, and nil on errors.
func getResize() *define.TerminalSize {
	winsize, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		logrus.Warnf("Could not get terminal size %v", err)
		return nil
	}
	return &define.TerminalSize{
		Width:  winsize.Width,
		Height: winsize.Height,
	}
}

// Helper for prepareAttach - set up a goroutine to generate terminal resize events
func resizeTty(ctx context.Context, resize chan define.TerminalSize) {
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
	return term.RestoreTerminal(os.Stdin.Fd(), state)
}

func handleTerminalAttach(ctx context.Context, resize chan define.TerminalSize) (context.CancelFunc, *term.State, error) {
	logrus.Debugf("Handling terminal attach")

	subCtx, cancel := context.WithCancel(ctx)

	resizeTty(subCtx, resize)

	oldTermState, err := term.SaveState(os.Stdin.Fd())
	if err != nil {
		// allow caller to not have to do any cleaning up if we error here
		cancel()
		return nil, nil, errors.Wrapf(err, "unable to save terminal state")
	}

	if _, err := term.SetRawTerminal(os.Stdin.Fd()); err != nil {
		return cancel, nil, err
	}

	return cancel, oldTermState, nil
}
