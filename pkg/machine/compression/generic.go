package compression

import (
	"io"
	"os"

	"github.com/containers/image/v5/pkg/compression"
	"github.com/sirupsen/logrus"
)

type genericDecompressor struct {
	compressedFilePath string
	compressedFile     *os.File
	uncompressStream   io.ReadCloser
}

func newGenericDecompressor(compressedFilePath string) decompressor {
	return &genericDecompressor{
		compressedFilePath: compressedFilePath,
	}
}

func (d *genericDecompressor) srcFilePath() string {
	return d.compressedFilePath
}

func (d *genericDecompressor) reader() (io.Reader, error) {
	srcFile, err := os.Open(d.compressedFilePath)
	if err != nil {
		return nil, err
	}
	d.compressedFile = srcFile
	return srcFile, nil
}

func (d *genericDecompressor) copy(w *os.File, r io.Reader) error {
	uncompressStream, _, err := compression.AutoDecompress(r)
	if err != nil {
		return err
	}
	d.uncompressStream = uncompressStream

	_, err = io.Copy(w, uncompressStream)
	return err
}

func (d *genericDecompressor) close() {
	if err := d.compressedFile.Close(); err != nil {
		logrus.Errorf("Unable to close compressed file: %q", err)
	}
	if err := d.uncompressStream.Close(); err != nil {
		logrus.Errorf("Unable to close uncompressed stream: %q", err)
	}
}
