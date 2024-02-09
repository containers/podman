package compression

import (
	"archive/zip"
	"errors"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

type zipDecompressor struct {
	compressedFilePath string
	zipReader          *zip.ReadCloser
	fileReader         io.ReadCloser
}

func newZipDecompressor(compressedFilePath string) decompressor {
	return &zipDecompressor{
		compressedFilePath: compressedFilePath,
	}
}

func (d *zipDecompressor) srcFilePath() string {
	return d.compressedFilePath
}

func (d *zipDecompressor) reader() (io.Reader, error) {
	zipReader, err := zip.OpenReader(d.compressedFilePath)
	if err != nil {
		return nil, err
	}
	d.zipReader = zipReader
	if len(zipReader.File) != 1 {
		return nil, errors.New("machine image files should consist of a single compressed file")
	}
	z, err := zipReader.File[0].Open()
	if err != nil {
		return nil, err
	}
	d.fileReader = z
	return z, nil
}

func (*zipDecompressor) copy(w *os.File, r io.Reader) error {
	_, err := io.Copy(w, r)
	return err
}

func (d *zipDecompressor) close() {
	if err := d.zipReader.Close(); err != nil {
		logrus.Errorf("Unable to close zip file: %q", err)
	}
	if err := d.fileReader.Close(); err != nil {
		logrus.Errorf("Unable to close zip file: %q", err)
	}
}
