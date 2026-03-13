package chunked

import (
	"io"

	"github.com/containers/storage/pkg/chunked/compressor"
	"github.com/containers/storage/pkg/chunked/internal"
)

const (
	TypeReg     = internal.TypeReg
	TypeChunk   = internal.TypeChunk
	TypeLink    = internal.TypeLink
	TypeChar    = internal.TypeChar
	TypeBlock   = internal.TypeBlock
	TypeDir     = internal.TypeDir
	TypeFifo    = internal.TypeFifo
	TypeSymlink = internal.TypeSymlink
)

// ZstdCompressor is a CompressorFunc for the zstd compression algorithm.
// Deprecated: Use pkg/chunked/compressor.ZstdCompressor.
func ZstdCompressor(r io.Writer, metadata map[string]string, level *int) (io.WriteCloser, error) {
	return compressor.ZstdCompressor(r, metadata, level)
}
