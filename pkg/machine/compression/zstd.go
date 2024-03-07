package compression

import (
	"io"

	"github.com/klauspost/compress/zstd"
)

type zstdDecompressor struct {
	genericDecompressor
}

func newZstdDecompressor(compressedFilePath string) (*zstdDecompressor, error) {
	d, err := newGenericDecompressor(compressedFilePath)
	return &zstdDecompressor{*d}, err
}

func (d *zstdDecompressor) decompress(w io.WriteSeeker, r io.Reader) error {
	zstdReader, err := zstd.NewReader(r)
	if err != nil {
		return err
	}
	defer zstdReader.Close()

	return d.sparseOptimizedCopy(w, zstdReader)
}
