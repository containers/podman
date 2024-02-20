package compression

import (
	"compress/gzip"
	"io"

	crcOs "github.com/crc-org/crc/v2/pkg/os"
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

func (d *gzipDecompressor) decompress(w io.WriteSeeker, r io.Reader) error {
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	d.gzReader = gzReader
	_, err = crcOs.CopySparse(w, gzReader)
	return err
}

func (d *gzipDecompressor) close() {
	if err := d.gzReader.Close(); err != nil {
		logrus.Errorf("Unable to close gz file: %q", err)
	}
	d.genericDecompressor.close()
}
