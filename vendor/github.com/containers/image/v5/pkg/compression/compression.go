package compression

import (
	"bytes"
	"compress/bzip2"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/containers/image/v5/pkg/compression/internal"
	"github.com/containers/image/v5/pkg/compression/types"
	"github.com/klauspost/pgzip"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
)

// Algorithm is a compression algorithm that can be used for CompressStream.
type Algorithm = types.Algorithm

var (
	// Gzip compression.
	Gzip = internal.NewAlgorithm("gzip", []byte{0x1F, 0x8B, 0x08}, GzipDecompressor, gzipCompressor)
	// Bzip2 compression.
	Bzip2 = internal.NewAlgorithm("bzip2", []byte{0x42, 0x5A, 0x68}, Bzip2Decompressor, bzip2Compressor)
	// Xz compression.
	Xz = internal.NewAlgorithm("Xz", []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00}, XzDecompressor, xzCompressor)
	// Zstd compression.
	Zstd = internal.NewAlgorithm("zstd", []byte{0x28, 0xb5, 0x2f, 0xfd}, ZstdDecompressor, zstdCompressor)

	compressionAlgorithms = map[string]Algorithm{
		Gzip.Name():  Gzip,
		Bzip2.Name(): Bzip2,
		Xz.Name():    Xz,
		Zstd.Name():  Zstd,
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
	return ioutil.NopCloser(bzip2.NewReader(r)), nil
}

// XzDecompressor is a DecompressorFunc for the xz compression algorithm.
func XzDecompressor(r io.Reader) (io.ReadCloser, error) {
	r, err := xz.NewReader(r)
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(r), nil
}

// gzipCompressor is a CompressorFunc for the gzip compression algorithm.
func gzipCompressor(r io.Writer, level *int) (io.WriteCloser, error) {
	if level != nil {
		return pgzip.NewWriterLevel(r, *level)
	}
	return pgzip.NewWriter(r), nil
}

// bzip2Compressor is a CompressorFunc for the bzip2 compression algorithm.
func bzip2Compressor(r io.Writer, level *int) (io.WriteCloser, error) {
	return nil, fmt.Errorf("bzip2 compression not supported")
}

// xzCompressor is a CompressorFunc for the xz compression algorithm.
func xzCompressor(r io.Writer, level *int) (io.WriteCloser, error) {
	return xz.NewWriter(r)
}

// CompressStream returns the compressor by its name
func CompressStream(dest io.Writer, algo Algorithm, level *int) (io.WriteCloser, error) {
	return internal.AlgorithmCompressor(algo)(dest, level)
}

// DetectCompressionFormat returns a DecompressorFunc if the input is recognized as a compressed format, nil otherwise.
// Because it consumes the start of input, other consumers must use the returned io.Reader instead to also read from the beginning.
func DetectCompressionFormat(input io.Reader) (Algorithm, DecompressorFunc, io.Reader, error) {
	buffer := [8]byte{}

	n, err := io.ReadAtLeast(input, buffer[:], len(buffer))
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		// This is a “real” error. We could just ignore it this time, process the data we have, and hope that the source will report the same error again.
		// Instead, fail immediately with the original error cause instead of a possibly secondary/misleading error returned later.
		return Algorithm{}, nil, nil, err
	}

	var retAlgo Algorithm
	var decompressor DecompressorFunc
	for _, algo := range compressionAlgorithms {
		if bytes.HasPrefix(buffer[:n], internal.AlgorithmPrefix(algo)) {
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
		return nil, false, errors.Wrapf(err, "Error detecting compression")
	}
	var res io.ReadCloser
	if decompressor != nil {
		res, err = decompressor(stream)
		if err != nil {
			return nil, false, errors.Wrapf(err, "Error initializing decompression")
		}
	} else {
		res = ioutil.NopCloser(stream)
	}
	return res, decompressor != nil, nil
}
