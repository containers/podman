package compression

import (
	"io"

	"github.com/klauspost/compress/zstd"
)

type zstdDecompressor struct {
	genericDecompressor
	zstdReader *zstd.Decoder
}

func newZstdDecompressor(compressedFilePath string) (*zstdDecompressor, error) {
	d, err := newGenericDecompressor(compressedFilePath)
	return &zstdDecompressor{*d, nil}, err
}

func (d *zstdDecompressor) decompress(w WriteSeekCloser, r io.Reader) error {
	zstdReader, err := zstd.NewReader(r)
	if err != nil {
		return err
	}
	d.zstdReader = zstdReader

	sparseWriter := NewSparseWriter(w)
	_, err = io.Copy(sparseWriter, zstdReader)
	return err
}

func (d *zstdDecompressor) close() {
	d.zstdReader.Close()
	d.genericDecompressor.close()
}
