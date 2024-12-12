package os

import (
	"bytes"
	"context"
	"io"
	"os"
)

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
		if _, err = CopySparse(context.TODO(), out, in); err != nil {
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

func CopySparse(ctx context.Context, dst io.WriteSeeker, src io.Reader) (int64, error) {
	copyBuf := make([]byte, copyChunkSize)

	if ctx == nil {
		panic("ctx is nil, this should not happen")
	}
	sparseWriter := newSparseWriter(ctx, dst)

	bytesWritten, err := io.CopyBuffer(sparseWriter, src, copyBuf)
	if err != nil {
		return bytesWritten, err
	}
	err = sparseWriter.Close()
	return bytesWritten, err
}

type sparseWriter struct {
	context         context.Context
	writer          io.WriteSeeker
	lastChunkSparse bool
}

func newSparseWriter(ctx context.Context, writer io.WriteSeeker) *sparseWriter {
	return &sparseWriter{context: ctx, writer: writer}
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
	select {
	case <-w.context.Done(): // Context cancelled
		return 0, w.context.Err()
	default:
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
