package compression

import (
	"io"

	"github.com/sirupsen/logrus"
)

type uncompressedDecompressor struct {
	genericDecompressor
}

func newUncompressedDecompressor(compressedFilePath string) (*uncompressedDecompressor, error) {
	d, err := newGenericDecompressor(compressedFilePath)
	return &uncompressedDecompressor{*d}, err
}

func (d *uncompressedDecompressor) decompress(w WriteSeekCloser, r io.Reader) error {
	sparseWriter := NewSparseWriter(w)
	defer func() {
		if err := sparseWriter.Close(); err != nil {
			logrus.Errorf("Unable to close uncompressed file: %q", err)
		}
	}()

	_, err := io.Copy(sparseWriter, r)
	return err
}
