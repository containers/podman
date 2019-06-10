// +build !windows

package cwriter

import (
	"io"
	"strings"
)

func (w *Writer) clearLines() error {
	_, err := io.WriteString(w.out, strings.Repeat(clearCursorAndLine, w.lineCount))
	return err
}
