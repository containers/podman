package compression

import (
	"bytes"
	"io"
	"os"
)

// TODO vendor this in ... pkg/os directory is small and code should be negligible
/*
	NOTE:   copy.go and copy.test were lifted from github.com/crc-org/crc because
			i was having trouble getting go to vendor it properly. all credit to them
*/

func copyFile(src, dst string, sparse bool) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	defer out.Close()

	if sparse {
		if _, err = CopySparse(out, in); err != nil {
			return err
		}
	} else {
		if _, err = io.Copy(out, in); err != nil {
			return err
		}
	}

	fi, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err = os.Chmod(dst, fi.Mode()); err != nil {
		return err
	}

	return out.Close()
}

func CopyFile(src, dst string) error {
	return copyFile(src, dst, false)
}

func CopyFileSparse(src, dst string) error {
	return copyFile(src, dst, true)
}

func CopySparse(dst io.WriteSeeker, src io.Reader) (int64, error) {
	copyBuf := make([]byte, copyChunkSize)
	sparseWriter := newSparseWriter(dst)

	bytesWritten, err := io.CopyBuffer(sparseWriter, src, copyBuf)
	if err != nil {
		return bytesWritten, err
	}
	err = sparseWriter.Close()
	return bytesWritten, err
}

type sparseWriter struct {
	writer          io.WriteSeeker
	lastChunkSparse bool
}

func newSparseWriter(writer io.WriteSeeker) *sparseWriter {
	return &sparseWriter{writer: writer}
}

const copyChunkSize = 4096

var emptyChunk = make([]byte, copyChunkSize)

func isEmptyChunk(p []byte) bool {
	// HasPrefix instead of bytes.Equal in order to handle the last chunk
	// of the file, which may be shorter than len(emptyChunk), and would
	// fail bytes.Equal()
	return bytes.HasPrefix(emptyChunk, p)
}

func (w *sparseWriter) Write(p []byte) (n int, err error) {
	if isEmptyChunk(p) {
		offset, err := w.writer.Seek(int64(len(p)), io.SeekCurrent)
		if err != nil {
			w.lastChunkSparse = false
			return 0, err
		}
		_ = offset
		w.lastChunkSparse = true
		return len(p), nil
	}
	w.lastChunkSparse = false
	return w.writer.Write(p)
}

func (w *sparseWriter) Close() error {
	if w.lastChunkSparse {
		if _, err := w.writer.Seek(-1, io.SeekCurrent); err != nil {
			return err
		}
		if _, err := w.writer.Write([]byte{0}); err != nil {
			return err
		}
	}
	return nil
}
