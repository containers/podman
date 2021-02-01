package blobinfocache

import (
	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
)

// FromBlobInfoCache returns a BlobInfoCache2 based on a BlobInfoCache, returning the original
// object if it implements BlobInfoCache2, or a wrapper which discards compression information
// if it only implements BlobInfoCache.
func FromBlobInfoCache(bic types.BlobInfoCache) BlobInfoCache2 {
	if bic2, ok := bic.(BlobInfoCache2); ok {
		return bic2
	}
	return &v1OnlyBlobInfoCache{
		BlobInfoCache: bic,
	}
}

type v1OnlyBlobInfoCache struct {
	types.BlobInfoCache
}

func (bic *v1OnlyBlobInfoCache) RecordDigestCompressorName(anyDigest digest.Digest, compressorName string) {
}

func (bic *v1OnlyBlobInfoCache) CandidateLocations2(transport types.ImageTransport, scope types.BICTransportScope, digest digest.Digest, canSubstitute bool) []BICReplacementCandidate2 {
	return nil
}

// CandidateLocationsFromV2 converts a slice of BICReplacementCandidate2 to a slice of
// types.BICReplacementCandidate, dropping compression information.
func CandidateLocationsFromV2(v2candidates []BICReplacementCandidate2) []types.BICReplacementCandidate {
	candidates := make([]types.BICReplacementCandidate, 0, len(v2candidates))
	for _, c := range v2candidates {
		candidates = append(candidates, types.BICReplacementCandidate{
			Digest:   c.Digest,
			Location: c.Location,
		})
	}
	return candidates
}

// OperationAndAlgorithmForCompressor returns CompressionOperation and CompressionAlgorithm
// values suitable for inclusion in a types.BlobInfo structure, based on the name of the
// compression algorithm, or Uncompressed, or UnknownCompression.  This is typically used by
// TryReusingBlob() implementations to set values in the BlobInfo structure that they return
// upon success.
func OperationAndAlgorithmForCompressor(compressorName string) (types.LayerCompression, *compression.Algorithm, error) {
	switch compressorName {
	case Uncompressed:
		return types.Decompress, nil, nil
	case UnknownCompression:
		return types.PreserveOriginal, nil, nil
	default:
		algo, err := compression.AlgorithmByName(compressorName)
		if err == nil {
			return types.Compress, &algo, nil
		}
		return types.PreserveOriginal, nil, err
	}
}
