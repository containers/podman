package impl

import (
	"github.com/containers/image/v5/internal/manifest"
	"github.com/containers/image/v5/internal/private"
	compression "github.com/containers/image/v5/pkg/compression/types"
	"golang.org/x/exp/slices"
)

// CandidateMatchesTryReusingBlobOptions validates if compression is required by the caller while selecting a blob, if it is required
// then function performs a match against the compression requested by the caller and compression of existing blob
// (which can be nil to represent uncompressed or unknown)
func CandidateMatchesTryReusingBlobOptions(options private.TryReusingBlobOptions, candidateCompression *compression.Algorithm) bool {
	if options.RequiredCompression != nil {
		if options.RequiredCompression.Name() == compression.ZstdChunkedAlgorithmName {
			// HACK: Never match when the caller asks for zstd:chunked, because we don’t record the annotations required to use the chunked blobs.
			// The caller must re-compress to build those annotations.
			return false
		}
		if candidateCompression == nil ||
			(options.RequiredCompression.Name() != candidateCompression.Name() && options.RequiredCompression.Name() != candidateCompression.BaseVariantName()) {
			return false
		}
	}

	// For candidateCompression == nil, we can’t tell the difference between “uncompressed” and “unknown”;
	// and “uncompressed” is acceptable in all known formats (well, it seems to work in practice for schema1),
	// so don’t impose any restrictions if candidateCompression == nil
	if options.PossibleManifestFormats != nil && candidateCompression != nil {
		if !slices.ContainsFunc(options.PossibleManifestFormats, func(mt string) bool {
			return manifest.MIMETypeSupportsCompressionAlgorithm(mt, *candidateCompression)
		}) {
			return false
		}
	}

	return true
}

func OriginalCandidateMatchesTryReusingBlobOptions(opts private.TryReusingBlobOptions) bool {
	return CandidateMatchesTryReusingBlobOptions(opts, opts.OriginalCompression)
}
