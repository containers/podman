package tarlog

import (
	"io"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/vbatts/tar-split/archive/tar"
)

type tarLogger struct {
	writer     *io.PipeWriter
	closeMutex *sync.Mutex
	stateMutex *sync.Mutex
	closed     bool
}

// NewLogger returns a writer that, when a tar archive is written to it, calls
// `logger` for each file header it encounters in the archive.
func NewLogger(logger func(*tar.Header)) (io.WriteCloser, error) {
	reader, writer := io.Pipe()
	t := &tarLogger{
		writer:     writer,
		closeMutex: new(sync.Mutex),
		stateMutex: new(sync.Mutex),
		closed:     false,
	}
	tr := tar.NewReader(reader)
	tr.RawAccounting = true
	t.closeMutex.Lock()
	go func() {
		hdr, err := tr.Next()
		for err == nil {
			logger(hdr)
			hdr, err = tr.Next()

		}
		// Make sure to avoid writes after the reader has been closed.
		t.stateMutex.Lock()
		t.closed = true
		if err := reader.Close(); err != nil {
			logrus.Errorf("error closing tarlogger reader: %v", err)
		}
		t.stateMutex.Unlock()
		// Unblock the Close().
		t.closeMutex.Unlock()
	}()
	return t, nil
}

func (t *tarLogger) Write(b []byte) (int, error) {
	t.stateMutex.Lock()
	if t.closed {
		// We cannot use os.Pipe() as this alters the tar's digest. Using
		// io.Pipe() requires this workaround as it does not allow for writes
		// after close.
		t.stateMutex.Unlock()
		return len(b), nil
	}
	t.stateMutex.Unlock()
	return t.writer.Write(b)
}

func (t *tarLogger) Close() error {
	err := t.writer.Close()
	// Wait for the reader to finish.
	t.closeMutex.Lock()
	return err
}
