package internal

import "io"

// CompressorFunc writes the compressed stream to the given writer using the specified compression level.
// The caller must call Close() on the stream (even if the input stream does not need closing!).
type CompressorFunc func(io.Writer, map[string]string, *int) (io.WriteCloser, error)

// DecompressorFunc returns the decompressed stream, given a compressed stream.
// The caller must call Close() on the decompressed stream (even if the compressed input stream does not need closing!).
type DecompressorFunc func(io.Reader) (io.ReadCloser, error)

// CanDecompressFunc returns true if the given stream can be decompressed using the specified algorithm.
type CanDecompressFunc func([]byte) bool

// Algorithm is a compression algorithm that can be used for CompressStream.
type Algorithm struct {
	name          string
	mime          string
	decompressor  DecompressorFunc
	compressor    CompressorFunc
	canDecompress CanDecompressFunc
}

// NewAlgorithm creates an Algorithm instance.
// This function exists so that Algorithm instances can only be created by code that
// is allowed to import this internal subpackage.
func NewAlgorithm(name, mime string, decompressor DecompressorFunc, compressor CompressorFunc, canDecompress CanDecompressFunc) Algorithm {
	return Algorithm{
		name:          name,
		mime:          mime,
		decompressor:  decompressor,
		compressor:    compressor,
		canDecompress: canDecompress,
	}
}

// Name returns the name for the compression algorithm.
func (c Algorithm) Name() string {
	return c.name
}

// InternalUnstableUndocumentedMIMEQuestionMark ???
// DO NOT USE THIS anywhere outside of c/image until it is properly documented.
func (c Algorithm) InternalUnstableUndocumentedMIMEQuestionMark() string {
	return c.mime
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

// AlgorithmCanDecompress returns the canDecompress field of algo.
// using the specified algorithm.
// This is a function instead of a public method so that it is only callable by code
// that is allowed to import this internal subpackage.
func AlgorithmCanDecompress(algo Algorithm) CanDecompressFunc {
	return algo.canDecompress
}
