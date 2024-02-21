package compression

import (
	"io"

	image "github.com/containers/image/v5/pkg/compression"
	"github.com/sirupsen/logrus"
)

type gzipDecompressor struct {
	genericDecompressor
	gzReader io.ReadCloser
}

func newGzipDecompressor(compressedFilePath string) (*gzipDecompressor, error) {
	d, err := newGenericDecompressor(compressedFilePath)
	return &gzipDecompressor{*d, nil}, err
}

func (d *gzipDecompressor) decompress(w WriteSeekCloser, r io.Reader) error {
	gzReader, err := image.GzipDecompressor(r)
	if err != nil {
		return err
	}
	d.gzReader = gzReader

	sparseWriter := NewSparseWriter(w)
	_, err = io.Copy(sparseWriter, gzReader)
	return err
}

func (d *gzipDecompressor) close() {
	if err := d.gzReader.Close(); err != nil {
		logrus.Errorf("Unable to close gz file: %q", err)
	}
	d.genericDecompressor.close()
}
