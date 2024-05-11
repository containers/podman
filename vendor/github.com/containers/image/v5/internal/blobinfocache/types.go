package blobinfocache

import (
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
)

const (
	// Uncompressed is the value we store in a blob info cache to indicate that we know that
	// the blob in the corresponding location is not compressed.
	Uncompressed = "uncompressed"
	// UnknownCompression is the value we store in a blob info cache to indicate that we don't
	// know if the blob in the corresponding location is compressed (and if so, how) or not.
	UnknownCompression = "unknown"
)

// BlobInfoCache2 extends BlobInfoCache by adding the ability to track information about what kind
// of compression was applied to the blobs it keeps information about.
type BlobInfoCache2 interface {
	types.BlobInfoCache

	// Open() sets up the cache for future accesses, potentially acquiring costly state. Each Open() must be paired with a Close().
	// Note that public callers may call the types.BlobInfoCache operations without Open()/Close().
	Open()
	// Close destroys state created by Open().
	Close()

	// RecordDigestCompressorName records a compressor for the blob with the specified digest,
	// or Uncompressed or UnknownCompression.
	// WARNING: Only call this with LOCALLY VERIFIED data; don’t record a compressor for a
	// digest just because some remote author claims so (e.g. because a manifest says so);
	// otherwise the cache could be poisoned and cause us to make incorrect edits to type
	// information in a manifest.
	RecordDigestCompressorName(anyDigest digest.Digest, compressorName string)
	// CandidateLocations2 returns a prioritized, limited, number of blobs and their locations (if known)
	// that could possibly be reused within the specified (transport scope) (if they still
	// exist, which is not guaranteed).
	//
	// If !canSubstitute, the returned candidates will match the submitted digest exactly; if
	// canSubstitute, data from previous RecordDigestUncompressedPair calls is used to also look
	// up variants of the blob which have the same uncompressed digest.
	//
	// The CompressorName fields in returned data must never be UnknownCompression.
	CandidateLocations2(transport types.ImageTransport, scope types.BICTransportScope, digest digest.Digest, canSubstitute bool) []BICReplacementCandidate2
}

// BICReplacementCandidate2 is an item returned by BlobInfoCache2.CandidateLocations2.
type BICReplacementCandidate2 struct {
	Digest          digest.Digest
	CompressorName  string                     // either the Name() of a known pkg/compression.Algorithm, or Uncompressed or UnknownCompression
	UnknownLocation bool                       // is true when `Location` for this blob is not set
	Location        types.BICLocationReference // not set if UnknownLocation is set to `true`
}
