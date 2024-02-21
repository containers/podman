package compression

import (
	"io"
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
	_, err := io.Copy(sparseWriter, r)
	return err
}
