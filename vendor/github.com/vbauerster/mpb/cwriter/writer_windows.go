// +build windows

package cwriter

import (
	"io"
	"strings"
	"syscall"
	"unsafe"

	isatty "github.com/mattn/go-isatty"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")

var (
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procSetConsoleCursorPosition   = kernel32.NewProc("SetConsoleCursorPosition")
	procFillConsoleOutputCharacter = kernel32.NewProc("FillConsoleOutputCharacterW")
	procFillConsoleOutputAttribute = kernel32.NewProc("FillConsoleOutputAttribute")
)

type (
	short int16
	word  uint16
	dword uint32

	coord struct {
		x short
		y short
	}
	smallRect struct {
		left   short
		top    short
		right  short
		bottom short
	}
	consoleScreenBufferInfo struct {
		size              coord
		cursorPosition    coord
		attributes        word
		window            smallRect
		maximumWindowSize coord
	}
)

// FdWriter is a writer with a file descriptor.
type FdWriter interface {
	io.Writer
	Fd() uintptr
}

func (w *Writer) clearLines() error {
	f, ok := w.out.(FdWriter)
	if ok && !isatty.IsTerminal(f.Fd()) {
		_, err := io.WriteString(w.out, strings.Repeat(clearCursorAndLine, w.lineCount))
		return err
	}
	fd := f.Fd()
	var info consoleScreenBufferInfo
	procGetConsoleScreenBufferInfo.Call(fd, uintptr(unsafe.Pointer(&info)))

	for i := 0; i < w.lineCount; i++ {
		// move the cursor up
		info.cursorPosition.y--
		procSetConsoleCursorPosition.Call(fd, uintptr(*(*int32)(unsafe.Pointer(&info.cursorPosition))))
		// clear the line
		cursor := coord{
			x: info.window.left,
			y: info.window.top + info.cursorPosition.y,
		}
		var count, w dword
		count = dword(info.size.x)
		procFillConsoleOutputCharacter.Call(fd, uintptr(' '), uintptr(count), *(*uintptr)(unsafe.Pointer(&cursor)), uintptr(unsafe.Pointer(&w)))
	}
	return nil
}
