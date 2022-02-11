//go:build windows
// +build windows

package goterm

import (
	"errors"
	"math"
	"os"

	"golang.org/x/sys/windows"
)

func getWinsize() (*winsize, error) {
	ws := new(winsize)
	fd := os.Stdout.Fd()
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info); err != nil {
		return nil, err
	}

	ws.Col = uint16(info.Window.Right - info.Window.Left + 1)
	ws.Row = uint16(info.Window.Bottom - info.Window.Top + 1)

	return ws, nil
}

// Height gets console height
func Height() int {
	ws, err := getWinsize()
	if err != nil {
		// returns math.MinInt32 if we could not retrieve the height of console window,
		// like VSCode debugging console
		if errors.Is(err, windows.WSAEOPNOTSUPP) {
			return math.MinInt32
		}
		return -1
	}
	return int(ws.Row)
}
