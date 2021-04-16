// +build windows

package cwriter

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var kernel32 = windows.NewLazySystemDLL("kernel32.dll")

var (
	procSetConsoleCursorPosition   = kernel32.NewProc("SetConsoleCursorPosition")
	procFillConsoleOutputCharacter = kernel32.NewProc("FillConsoleOutputCharacterW")
)

func (w *Writer) clearLines() error {
	if !w.isTerminal {
		// hope it's cygwin or similar
		return w.ansiCuuAndEd()
	}

	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(w.fd), &info); err != nil {
		return err
	}

	info.CursorPosition.Y -= int16(w.lineCount)
	if info.CursorPosition.Y < 0 {
		info.CursorPosition.Y = 0
	}
	_, _, _ = procSetConsoleCursorPosition.Call(
		uintptr(w.fd),
		uintptr(uint32(uint16(info.CursorPosition.Y))<<16|uint32(uint16(info.CursorPosition.X))),
	)

	// clear the lines
	cursor := &windows.Coord{
		X: info.Window.Left,
		Y: info.CursorPosition.Y,
	}
	count := uint32(info.Size.X) * uint32(w.lineCount)
	_, _, _ = procFillConsoleOutputCharacter.Call(
		uintptr(w.fd),
		uintptr(' '),
		uintptr(count),
		*(*uintptr)(unsafe.Pointer(cursor)),
		uintptr(unsafe.Pointer(new(uint32))),
	)
	return nil
}

// GetSize returns the visible dimensions of the given terminal.
//
// These dimensions don't include any scrollback buffer height.
func GetSize(fd int) (width, height int, err error) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info); err != nil {
		return 0, 0, err
	}
	// terminal.GetSize from crypto/ssh adds "+ 1" to both width and height:
	// https://go.googlesource.com/crypto/+/refs/heads/release-branch.go1.14/ssh/terminal/util_windows.go#75
	// but looks like this is a root cause of issue #66, so removing both "+ 1" have fixed it.
	return int(info.Window.Right - info.Window.Left), int(info.Window.Bottom - info.Window.Top), nil
}

// IsTerminal returns whether the given file descriptor is a terminal.
func IsTerminal(fd int) bool {
	var st uint32
	err := windows.GetConsoleMode(windows.Handle(fd), &st)
	return err == nil
}
