package compression

import (
	"io"
	"os"

	crcOs "github.com/crc-org/crc/v2/pkg/os"
	"github.com/sirupsen/logrus"
)

type uncompressedDecompressor struct {
	compressedFilePath string
	compressedFile     *os.File
}

func newUncompressedDecompressor(compressedFilePath string) decompressor {
	return &uncompressedDecompressor{
		compressedFilePath: compressedFilePath,
	}
}

func (d *uncompressedDecompressor) srcFilePath() string {
	return d.compressedFilePath
}

func (d *uncompressedDecompressor) reader() (io.Reader, error) {
	srcFile, err := os.Open(d.compressedFilePath)
	if err != nil {
		return nil, err
	}
	d.compressedFile = srcFile

	return srcFile, nil
}

func (*uncompressedDecompressor) copy(w *os.File, r io.Reader) error {
	_, err := crcOs.CopySparse(w, r)
	return err
}

func (d *uncompressedDecompressor) close() {
	if err := d.compressedFile.Close(); err != nil {
		logrus.Errorf("Unable to close gz file: %q", err)
	}
}
