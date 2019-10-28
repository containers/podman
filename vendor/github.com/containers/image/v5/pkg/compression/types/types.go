package types

import (
	"github.com/containers/image/v5/pkg/compression/internal"
)

// DecompressorFunc returns the decompressed stream, given a compressed stream.
// The caller must call Close() on the decompressed stream (even if the compressed input stream does not need closing!).
type DecompressorFunc = internal.DecompressorFunc

// Algorithm is a compression algorithm provided and supported by pkg/compression.
// It can’t be supplied from the outside.
type Algorithm = internal.Algorithm
