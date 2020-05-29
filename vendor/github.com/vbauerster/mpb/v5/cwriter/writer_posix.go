// +build !windows

package cwriter

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func (w *Writer) clearLines() {
	fmt.Fprintf(w.out, cuuAndEd, w.lineCount)
}

// GetSize returns the dimensions of the given terminal.
func GetSize(fd uintptr) (width, height int, err error) {
	ws, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	if err != nil {
		return -1, -1, err
	}
	return int(ws.Col), int(ws.Row), nil
}
