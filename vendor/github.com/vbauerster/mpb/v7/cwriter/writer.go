package cwriter

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
)

// ErrNotTTY not a TeleTYpewriter error.
var ErrNotTTY = errors.New("not a terminal")

// https://github.com/dylanaraps/pure-sh-bible#cursor-movement
const (
	escOpen  = "\x1b["
	cuuAndEd = "A\x1b[J"
)

// Writer is a buffered the writer that updates the terminal. The
// contents of writer will be flushed when Flush is called.
type Writer struct {
	out      io.Writer
	buf      bytes.Buffer
	lines    int // how much lines to clear before flushing new ones
	fd       int
	terminal bool
	termSize func(int) (int, int, error)
}

// New returns a new Writer with defaults.
func New(out io.Writer) *Writer {
	w := &Writer{
		out: out,
		termSize: func(_ int) (int, int, error) {
			return -1, -1, ErrNotTTY
		},
	}
	if f, ok := out.(*os.File); ok {
		w.fd = int(f.Fd())
		if IsTerminal(w.fd) {
			w.terminal = true
			w.termSize = func(fd int) (int, int, error) {
				return GetSize(fd)
			}
		}
	}
	return w
}

// Flush flushes the underlying buffer.
func (w *Writer) Flush(lines int) (err error) {
	// some terminals interpret 'cursor up 0' as 'cursor up 1'
	if w.lines > 0 {
		err = w.clearLines()
		if err != nil {
			return
		}
	}
	w.lines = lines
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

// GetTermSize returns WxH of underlying terminal.
func (w *Writer) GetTermSize() (width, height int, err error) {
	return w.termSize(w.fd)
}

func (w *Writer) ansiCuuAndEd() error {
	buf := make([]byte, 8)
	buf = strconv.AppendInt(buf[:copy(buf, escOpen)], int64(w.lines), 10)
	_, err := w.out.Write(append(buf, cuuAndEd...))
	return err
}
