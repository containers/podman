package internal

import (
	"io"

	"github.com/opencontainers/go-digest"
)

// CompressorFunc writes the compressed stream to the given writer using the specified compression level.
// The metadata value is filled by the compressor, and can be used to store information about
// the compressed stream. The metadata values which are meaningful for _this_ layer data
// _may_ be _copied_ from a previous layer (passed to this compressor), but that is not guaranteed,
// and the metadata is going to be recorded for this layer, so this function should set metadata
// values only for the current layer, not for any old / reused layer data.
// Metadata keys must be unique within a single layer, and include a namespace, e.g. gzip stores
// the header information in metadata["gzip.header"] = base64(header), in a compressed tar stream.
//
// As for the general file's digest metadata, it is the caller's responsibility to set the
// relevant information in annotations or elsewhere — this metadata is expected to be more layer-specific,
// and the caller will have better visibility to ensure that the metadata ends up in the correct layer digest
// of the compressed stream against unwanted modification. (In OCI container images, this metadata
// is usually carried in manifest annotations.)
//
// If the compression generates such metadata, it is written to the provided metadata map.
//
// The caller must call Close() on the stream (even if the input stream does not need closing!).
type CompressorFunc func(io.Writer, map[string]string, *int, digest.Algorithm) (io.WriteCloser, error)

// DecompressorFunc returns the decompressed stream, given a compressed stream.
// The caller must call Close() on the decompressed stream (even if the compressed input stream does not need closing!).
type DecompressorFunc func(io.Reader) (io.ReadCloser, error)

// Algorithm is a compression algorithm that can be used for CompressStream.
type Algorithm struct {
	name            string
	baseVariantName string
	prefix          []byte // Initial bytes of a stream compressed using this algorithm, or empty to disable detection.
	decompressor    DecompressorFunc
	compressor      CompressorFunc
}

// NewAlgorithm creates an Algorithm instance.
// nontrivialBaseVariantName is typically "".
// This function exists so that Algorithm instances can only be created by code that
// is allowed to import this internal subpackage.
func NewAlgorithm(name, nontrivialBaseVariantName string, prefix []byte, decompressor DecompressorFunc, compressor CompressorFunc) Algorithm {
	baseVariantName := name
	if nontrivialBaseVariantName != "" {
		baseVariantName = nontrivialBaseVariantName
	}
	return Algorithm{
		name:            name,
		baseVariantName: baseVariantName,
		prefix:          prefix,
		decompressor:    decompressor,
		compressor:      compressor,
	}
}

// Name returns the name for the compression algorithm.
func (c Algorithm) Name() string {
	return c.name
}

// BaseVariantName returns the name of the “base variant” of the compression algorithm.
// It is either equal to Name() of the same algorithm, or equal to Name() of some other Algorithm (the “base variant”).
// This supports a single level of “is-a” relationship between compression algorithms, e.g. where "zstd:chunked" data is valid "zstd" data.
func (c Algorithm) BaseVariantName() string {
	return c.baseVariantName
}

// AlgorithmCompressor returns the compressor field of algo.
// This is a function instead of a public method so that it is only callable by code
// that is allowed to import this internal subpackage.
func AlgorithmCompressor(algo Algorithm) CompressorFunc {
	return algo.compressor
}

// AlgorithmDecompressor returns the decompressor field of algo.
// This is a function instead of a public method so that it is only callable by code
// that is allowed to import this internal subpackage.
func AlgorithmDecompressor(algo Algorithm) DecompressorFunc {
	return algo.decompressor
}

// AlgorithmPrefix returns the prefix field of algo.
// This is a function instead of a public method so that it is only callable by code
// that is allowed to import this internal subpackage.
func AlgorithmPrefix(algo Algorithm) []byte {
	return algo.prefix
}
