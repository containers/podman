package tarlog

import (
	"archive/tar"
	"io"
	"os"
	"sync"

	"github.com/pkg/errors"
)

type tarLogger struct {
	writer *os.File
	wg     sync.WaitGroup
}

// NewLogger returns a writer that, when a tar archive is written to it, calls
// `logger` for each file header it encounters in the archive.
func NewLogger(logger func(*tar.Header)) (io.WriteCloser, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, errors.Wrapf(err, "error creating pipe for tar logger")
	}
	t := &tarLogger{writer: writer}
	tr := tar.NewReader(reader)
	t.wg.Add(1)
	go func() {
		hdr, err := tr.Next()
		for err == nil {
			logger(hdr)
			hdr, err = tr.Next()
		}
		reader.Close()
		t.wg.Done()
	}()
	return t, nil
}

func (t *tarLogger) Write(b []byte) (int, error) {
	return t.writer.Write(b)
}

func (t *tarLogger) Close() error {
	err := t.writer.Close()
	t.wg.Wait()
	return err
}
