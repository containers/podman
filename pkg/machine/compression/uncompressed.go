package compression

import (
	"io"

	crcOs "github.com/crc-org/crc/v2/pkg/os"
)

type uncompressedDecompressor struct {
	genericDecompressor
}

func newUncompressedDecompressor(compressedFilePath string) (*uncompressedDecompressor, error) {
	d, err := newGenericDecompressor(compressedFilePath)
	return &uncompressedDecompressor{*d}, err
}

func (*uncompressedDecompressor) decompress(w WriteSeekCloser, r io.Reader) error {
	_, err := crcOs.CopySparse(w, r)
	return err
}
