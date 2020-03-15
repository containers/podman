// +build !windows

package cwriter

import "fmt"

func (w *Writer) clearLines() {
	fmt.Fprintf(w.out, cuuAndEd, w.lineCount)
}
