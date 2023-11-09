package compression

import (
	"bytes"
	"compress/bzip2"
	"fmt"
	"io"

	"github.com/containers/image/v5/pkg/compression/internal"
	"github.com/containers/image/v5/pkg/compression/types"
	"github.com/containers/storage/pkg/chunked/compressor"
	"github.com/klauspost/pgzip"
	"github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
)

// Algorithm is a compression algorithm that can be used for CompressStream.
type Algorithm = types.Algorithm

func hasPrefix(buffer []byte, prefix []byte) bool {
	return bytes.HasPrefix(buffer, prefix)
}

var (
	// Gzip compression.
	Gzip = internal.NewAlgorithm(types.GzipAlgorithmName, types.GzipAlgorithmName,
		GzipDecompressor, gzipCompressor, func(x []byte) bool {
			return hasPrefix(x, []byte{0x1F, 0x8B, 0x08})
		})
	// Bzip2 compression.
	Bzip2 = internal.NewAlgorithm(types.Bzip2AlgorithmName, types.Bzip2AlgorithmName,
		Bzip2Decompressor, bzip2Compressor, func(x []byte) bool {
			return hasPrefix(x, []byte{0x42, 0x5A, 0x68})
		})
	// Xz compression.
	Xz = internal.NewAlgorithm(types.XzAlgorithmName, types.XzAlgorithmName,
		XzDecompressor, xzCompressor, func(x []byte) bool {
			return hasPrefix(x, []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00})
		})

	// zstdChunkedPrefix is the magic prefix for zstd chunked files.
	zstdChunkedPrefix = []byte{0x28, 0xb5, 0x2f, 0xfd, 0x04, 0x70, 0x01, 0x00, 0x00, 0x99, 0xe9, 0xd8, 0x51, 0x50, 0x2a, 0x4d, 0x18, 0x0c, 0x00, 0x00, 0x00, 0x7a, 0x73, 0x74, 0x64, 0x3a, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x65, 0x64}

	// Zstd compression.
	Zstd = internal.NewAlgorithm(types.ZstdAlgorithmName, types.ZstdAlgorithmName,
		ZstdDecompressor, zstdCompressor, func(x []byte) bool {
			return hasPrefix(x, []byte{0x28, 0xb5, 0x2f, 0xfd}) && !hasPrefix(x, zstdChunkedPrefix)
		})
	// ZstdChunked is a Zstd compression with chunk metadta which allows random access to individual files.
	ZstdChunked = internal.NewAlgorithm(types.ZstdChunkedAlgorithmName, types.ZstdAlgorithmName, /* Note: InternalUnstableUndocumentedMIMEQuestionMark is not ZstdChunkedAlgorithmName */
		ZstdDecompressor, compressor.ZstdCompressor, func(x []byte) bool {
			return hasPrefix(x, zstdChunkedPrefix)
		})

	compressionAlgorithms = map[string]Algorithm{
		Gzip.Name():        Gzip,
		Bzip2.Name():       Bzip2,
		Xz.Name():          Xz,
		Zstd.Name():        Zstd,
		ZstdChunked.Name(): ZstdChunked,
	}
)

// AlgorithmByName returns the compressor by its name
func AlgorithmByName(name string) (Algorithm, error) {
	algorithm, ok := compressionAlgorithms[name]
	if ok {
		return algorithm, nil
	}
	return Algorithm{}, fmt.Errorf("cannot find compressor for %q", name)
}

// DecompressorFunc returns the decompressed stream, given a compressed stream.
// The caller must call Close() on the decompressed stream (even if the compressed input stream does not need closing!).
type DecompressorFunc = internal.DecompressorFunc

// GzipDecompressor is a DecompressorFunc for the gzip compression algorithm.
func GzipDecompressor(r io.Reader) (io.ReadCloser, error) {
	return pgzip.NewReader(r)
}

// Bzip2Decompressor is a DecompressorFunc for the bzip2 compression algorithm.
func Bzip2Decompressor(r io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(bzip2.NewReader(r)), nil
}

// XzDecompressor is a DecompressorFunc for the xz compression algorithm.
func XzDecompressor(r io.Reader) (io.ReadCloser, error) {
	r, err := xz.NewReader(r)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(r), nil
}

// gzipCompressor is a CompressorFunc for the gzip compression algorithm.
func gzipCompressor(r io.Writer, metadata map[string]string, level *int) (io.WriteCloser, error) {
	if level != nil {
		return pgzip.NewWriterLevel(r, *level)
	}
	return pgzip.NewWriter(r), nil
}

// bzip2Compressor is a CompressorFunc for the bzip2 compression algorithm.
func bzip2Compressor(r io.Writer, metadata map[string]string, level *int) (io.WriteCloser, error) {
	return nil, fmt.Errorf("bzip2 compression not supported")
}

// xzCompressor is a CompressorFunc for the xz compression algorithm.
func xzCompressor(r io.Writer, metadata map[string]string, level *int) (io.WriteCloser, error) {
	return xz.NewWriter(r)
}

// CompressStream returns the compressor by its name
func CompressStream(dest io.Writer, algo Algorithm, level *int) (io.WriteCloser, error) {
	m := map[string]string{}
	return internal.AlgorithmCompressor(algo)(dest, m, level)
}

// CompressStreamWithMetadata returns the compressor by its name.  If the compression
// generates any metadata, it is written to the provided metadata map.
func CompressStreamWithMetadata(dest io.Writer, metadata map[string]string, algo Algorithm, level *int) (io.WriteCloser, error) {
	return internal.AlgorithmCompressor(algo)(dest, metadata, level)
}

// DetectCompressionFormat returns an Algorithm and DecompressorFunc if the input is recognized as a compressed format, an invalid
// value and nil otherwise.
// Because it consumes the start of input, other consumers must use the returned io.Reader instead to also read from the beginning.
func DetectCompressionFormat(input io.Reader) (Algorithm, DecompressorFunc, io.Reader, error) {
	buffer := [36]byte{}

	n, err := io.ReadAtLeast(input, buffer[:], len(buffer))
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		// This is a “real” error. We could just ignore it this time, process the data we have, and hope that the source will report the same error again.
		// Instead, fail immediately with the original error cause instead of a possibly secondary/misleading error returned later.
		return Algorithm{}, nil, nil, err
	}

	var retAlgo Algorithm
	var decompressor DecompressorFunc
	for _, algo := range compressionAlgorithms {
		canDecompress := internal.AlgorithmCanDecompress(algo)
		if canDecompress != nil && canDecompress(buffer[:n]) {
			logrus.Debugf("Detected compression format %s", algo.Name())
			retAlgo = algo
			decompressor = internal.AlgorithmDecompressor(algo)
			break
		}
	}
	if decompressor == nil {
		logrus.Debugf("No compression detected")
	}

	return retAlgo, decompressor, io.MultiReader(bytes.NewReader(buffer[:n]), input), nil
}

// DetectCompression returns a DecompressorFunc if the input is recognized as a compressed format, nil otherwise.
// Because it consumes the start of input, other consumers must use the returned io.Reader instead to also read from the beginning.
func DetectCompression(input io.Reader) (DecompressorFunc, io.Reader, error) {
	_, d, r, e := DetectCompressionFormat(input)
	return d, r, e
}

// AutoDecompress takes a stream and returns an uncompressed version of the
// same stream.
// The caller must call Close() on the returned stream (even if the input does not need,
// or does not even support, closing!).
func AutoDecompress(stream io.Reader) (io.ReadCloser, bool, error) {
	decompressor, stream, err := DetectCompression(stream)
	if err != nil {
		return nil, false, fmt.Errorf("detecting compression: %w", err)
	}
	var res io.ReadCloser
	if decompressor != nil {
		res, err = decompressor(stream)
		if err != nil {
			return nil, false, fmt.Errorf("initializing decompression: %w", err)
		}
	} else {
		res = io.NopCloser(stream)
	}
	return res, decompressor != nil, nil
}
