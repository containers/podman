package impl

import (
	"github.com/containers/image/v5/internal/private"
	compression "github.com/containers/image/v5/pkg/compression/types"
)

// BlobMatchesRequiredCompression validates if compression is required by the caller while selecting a blob, if it is required
// then function performs a match against the compression requested by the caller and compression of existing blob
// (which can be nil to represent uncompressed or unknown)
func BlobMatchesRequiredCompression(options private.TryReusingBlobOptions, candidateCompression *compression.Algorithm) bool {
	if options.RequiredCompression == nil {
		return true // no requirement imposed
	}
	if options.RequiredCompression.Name() == compression.ZstdChunkedAlgorithmName {
		// HACK: Never match when the caller asks for zstd:chunked, because we don’t record the annotations required to use the chunked blobs.
		// The caller must re-compress to build those annotations.
		return false
	}
	return candidateCompression != nil && (options.RequiredCompression.Name() == candidateCompression.Name())
}

func OriginalBlobMatchesRequiredCompression(opts private.TryReusingBlobOptions) bool {
	return BlobMatchesRequiredCompression(opts, opts.OriginalCompression)
}
