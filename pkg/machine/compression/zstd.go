package compression

import (
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/sirupsen/logrus"
)

type zstdDecompressor struct {
	genericDecompressor
}

func newZstdDecompressor(compressedFilePath string) (*zstdDecompressor, error) {
	d, err := newGenericDecompressor(compressedFilePath)
	return &zstdDecompressor{*d}, err
}

func (d *zstdDecompressor) decompress(w WriteSeekCloser, r io.Reader) error {
	zstdReader, err := zstd.NewReader(r)
	if err != nil {
		return err
	}
	defer zstdReader.Close()

	sparseWriter := NewSparseWriter(w)
	defer func() {
		if err := sparseWriter.Close(); err != nil {
			logrus.Errorf("Unable to close uncompressed file: %q", err)
		}
	}()

	_, err = io.Copy(sparseWriter, zstdReader)
	return err
}
