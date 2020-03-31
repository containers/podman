package cwriter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

// NotATTY not a TeleTYpewriter error.
var NotATTY = errors.New("not a terminal")

var cuuAndEd = fmt.Sprintf("%c[%%dA%[1]c[J", 27)

// Writer is a buffered the writer that updates the terminal. The
// contents of writer will be flushed when Flush is called.
type Writer struct {
	out        io.Writer
	buf        bytes.Buffer
	lineCount  int
	fd         uintptr
	isTerminal bool
}

// New returns a new Writer with defaults.
func New(out io.Writer) *Writer {
	w := &Writer{out: out}
	if f, ok := out.(*os.File); ok {
		w.fd = f.Fd()
		w.isTerminal = terminal.IsTerminal(int(w.fd))
	}
	return w
}

// Flush flushes the underlying buffer.
func (w *Writer) Flush(lineCount int) (err error) {
	if w.lineCount > 0 {
		w.clearLines()
	}
	w.lineCount = lineCount
	_, err = w.buf.WriteTo(w.out)
	return
}

// Write appends the contents of p to the underlying buffer.
func (w *Writer) Write(p []byte) (n int, err error) {
	return w.buf.Write(p)
}

// WriteString writes string to the underlying buffer.
func (w *Writer) WriteString(s string) (n int, err error) {
	return w.buf.WriteString(s)
}

// ReadFrom reads from the provided io.Reader and writes to the
// underlying buffer.
func (w *Writer) ReadFrom(r io.Reader) (n int64, err error) {
	return w.buf.ReadFrom(r)
}

// GetWidth returns width of underlying terminal.
func (w *Writer) GetWidth() (int, error) {
	if w.isTerminal {
		tw, _, err := terminal.GetSize(int(w.fd))
		return tw, err
	}
	return -1, NotATTY
}
