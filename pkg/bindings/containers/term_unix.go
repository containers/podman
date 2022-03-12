//go:build !windows
// +build !windows

package containers

import (
	"context"
	"os"
	"os/signal"

	sig "github.com/containers/podman/v4/pkg/signal"
	"golang.org/x/crypto/ssh/terminal"
)

func makeRawTerm(stdin *os.File) (*terminal.State, error) {
	return terminal.MakeRaw(int(stdin.Fd()))
}

func notifyWinChange(ctx context.Context, winChange chan os.Signal, stdin *os.File, stdout *os.File) {
	signal.Notify(winChange, sig.SIGWINCH)
}

func getTermSize(stdin *os.File, stdout *os.File) (width, height int, err error) {
	return terminal.GetSize(int(stdin.Fd()))
}
