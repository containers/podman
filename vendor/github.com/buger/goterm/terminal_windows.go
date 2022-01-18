// +build windows

package goterm

import (
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
