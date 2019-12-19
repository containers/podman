// +build windows

package cwriter

import (
	"fmt"
	"syscall"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")

var (
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procSetConsoleCursorPosition   = kernel32.NewProc("SetConsoleCursorPosition")
	procFillConsoleOutputCharacter = kernel32.NewProc("FillConsoleOutputCharacterW")
	procFillConsoleOutputAttribute = kernel32.NewProc("FillConsoleOutputAttribute")
)

type coord struct {
	x int16
	y int16
}

type smallRect struct {
	left   int16
	top    int16
	right  int16
	bottom int16
}

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        uint16
	window            smallRect
	maximumWindowSize coord
}

func (w *Writer) clearLines() {
	if !w.isTerminal {
		fmt.Fprintf(w.out, cuuAndEd, w.lineCount)
	}
	var info consoleScreenBufferInfo
	procGetConsoleScreenBufferInfo.Call(w.fd, uintptr(unsafe.Pointer(&info)))

	info.cursorPosition.y -= int16(w.lineCount)
	if info.cursorPosition.y < 0 {
		info.cursorPosition.y = 0
	}
	procSetConsoleCursorPosition.Call(w.fd, uintptr(uint32(uint16(info.cursorPosition.y))<<16|uint32(uint16(info.cursorPosition.x))))

	// clear the lines
	cursor := coord{
		x: info.window.left,
		y: info.cursorPosition.y,
	}
	count := uint32(info.size.x) * uint32(w.lineCount)
	procFillConsoleOutputCharacter.Call(w.fd, uintptr(' '), uintptr(count), *(*uintptr)(unsafe.Pointer(&cursor)), uintptr(unsafe.Pointer(new(uint32))))
}
