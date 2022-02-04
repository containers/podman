//go:build !windows && !plan9 && !solaris
// +build !windows,!plan9,!solaris

package goterm

import (
	"errors"
	"math"
	"os"

	"golang.org/x/sys/unix"
)

func getWinsize() (*unix.Winsize, error) {

	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return nil, os.NewSyscallError("GetWinsize", err)
	}

	return ws, nil
}

// Height gets console height
func Height() int {
	ws, err := getWinsize()
	if err != nil {
		// returns math.MinInt32 if we could not retrieve the height of console window,
		// like VSCode debugging console
		if errors.Is(err, unix.EOPNOTSUPP) {
			return math.MinInt32
		}
		return -1
	}
	return int(ws.Row)
}
