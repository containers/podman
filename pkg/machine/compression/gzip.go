package compression

import (
	"compress/gzip"
	"io"
	"os"

	crcOs "github.com/crc-org/crc/v2/pkg/os"
	"github.com/sirupsen/logrus"
)

type gzDecompressor struct {
	compressedFilePath string
	compressedFile     *os.File
	gzReader           *gzip.Reader
}

func newGzipDecompressor(compressedFilePath string) decompressor {
	return &gzDecompressor{
		compressedFilePath: compressedFilePath,
	}
}

func (d *gzDecompressor) srcFilePath() string {
	return d.compressedFilePath
}

func (d *gzDecompressor) reader() (io.Reader, error) {
	srcFile, err := os.Open(d.compressedFilePath)
	if err != nil {
		return nil, err
	}
	d.compressedFile = srcFile

	gzReader, err := gzip.NewReader(srcFile)
	if err != nil {
		return gzReader, err
	}
	d.gzReader = gzReader

	return gzReader, nil
}

func (*gzDecompressor) copy(w *os.File, r io.Reader) error {
	_, err := crcOs.CopySparse(w, r)
	return err
}

func (d *gzDecompressor) close() {
	if err := d.compressedFile.Close(); err != nil {
		logrus.Errorf("Unable to close gz file: %q", err)
	}
	if err := d.gzReader.Close(); err != nil {
		logrus.Errorf("Unable to close gz file: %q", err)
	}
}
