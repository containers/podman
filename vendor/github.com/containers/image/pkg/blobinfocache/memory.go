package blobinfocache

import (
	"time"

	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

// locationKey only exists to make lookup in knownLocations easier.
type locationKey struct {
	transport  string
	scope      types.BICTransportScope
	blobDigest digest.Digest
}

// memoryCache implements an in-memory-only BlobInfoCache
type memoryCache struct {
	uncompressedDigests   map[digest.Digest]digest.Digest
	digestsByUncompressed map[digest.Digest]map[digest.Digest]struct{}             // stores a set of digests for each uncompressed digest
	knownLocations        map[locationKey]map[types.BICLocationReference]time.Time // stores last known existence time for each location reference
}

// NewMemoryCache returns a BlobInfoCache implementation which is in-memory only.
// This is primarily intended for tests, but also used as a fallback if DefaultCache
// can’t determine, or set up, the location for a persistent cache.
// Manual users of types.{ImageSource,ImageDestination} might also use this instead of a persistent cache.
func NewMemoryCache() types.BlobInfoCache {
	return &memoryCache{
		uncompressedDigests:   map[digest.Digest]digest.Digest{},
		digestsByUncompressed: map[digest.Digest]map[digest.Digest]struct{}{},
		knownLocations:        map[locationKey]map[types.BICLocationReference]time.Time{},
	}
}

// UncompressedDigest returns an uncompressed digest corresponding to anyDigest.
// May return anyDigest if it is known to be uncompressed.
// Returns "" if nothing is known about the digest (it may be compressed or uncompressed).
func (mem *memoryCache) UncompressedDigest(anyDigest digest.Digest) digest.Digest {
	if d, ok := mem.uncompressedDigests[anyDigest]; ok {
		return d
	}
	// Presence in digestsByUncompressed implies that anyDigest must already refer to an uncompressed digest.
	// This way we don't have to waste storage space with trivial (uncompressed, uncompressed) mappings
	// when we already record a (compressed, uncompressed) pair.
	if m, ok := mem.digestsByUncompressed[anyDigest]; ok && len(m) > 0 {
		return anyDigest
	}
	return ""
}

// RecordDigestUncompressedPair records that the uncompressed version of anyDigest is uncompressed.
// It’s allowed for anyDigest == uncompressed.
// WARNING: Only call this for LOCALLY VERIFIED data; don’t record a digest pair just because some remote author claims so (e.g.
// because a manifest/config pair exists); otherwise the cache could be poisoned and allow substituting unexpected blobs.
// (Eventually, the DiffIDs in image config could detect the substitution, but that may be too late, and not all image formats contain that data.)
func (mem *memoryCache) RecordDigestUncompressedPair(anyDigest digest.Digest, uncompressed digest.Digest) {
	if previous, ok := mem.uncompressedDigests[anyDigest]; ok && previous != uncompressed {
		logrus.Warnf("Uncompressed digest for blob %s previously recorded as %s, now %s", anyDigest, previous, uncompressed)
	}
	mem.uncompressedDigests[anyDigest] = uncompressed

	anyDigestSet, ok := mem.digestsByUncompressed[uncompressed]
	if !ok {
		anyDigestSet = map[digest.Digest]struct{}{}
		mem.digestsByUncompressed[uncompressed] = anyDigestSet
	}
	anyDigestSet[anyDigest] = struct{}{} // Possibly writing the same struct{}{} presence marker again.
}

// RecordKnownLocation records that a blob with the specified digest exists within the specified (transport, scope) scope,
// and can be reused given the opaque location data.
func (mem *memoryCache) RecordKnownLocation(transport types.ImageTransport, scope types.BICTransportScope, blobDigest digest.Digest, location types.BICLocationReference) {
	key := locationKey{transport: transport.Name(), scope: scope, blobDigest: blobDigest}
	locationScope, ok := mem.knownLocations[key]
	if !ok {
		locationScope = map[types.BICLocationReference]time.Time{}
		mem.knownLocations[key] = locationScope
	}
	locationScope[location] = time.Now() // Possibly overwriting an older entry.
}

// appendReplacementCandiates creates candidateWithTime values for (transport, scope, digest), and returns the result of appending them to candidates.
func (mem *memoryCache) appendReplacementCandidates(candidates []candidateWithTime, transport types.ImageTransport, scope types.BICTransportScope, digest digest.Digest) []candidateWithTime {
	locations := mem.knownLocations[locationKey{transport: transport.Name(), scope: scope, blobDigest: digest}] // nil if not present
	for l, t := range locations {
		candidates = append(candidates, candidateWithTime{
			candidate: types.BICReplacementCandidate{
				Digest:   digest,
				Location: l,
			},
			lastSeen: t,
		})
	}
	return candidates
}

// CandidateLocations returns a prioritized, limited, number of blobs and their locations that could possibly be reused
// within the specified (transport scope) (if they still exist, which is not guaranteed).
//
// If !canSubstitute, the returned cadidates will match the submitted digest exactly; if canSubstitute,
// data from previous RecordDigestUncompressedPair calls is used to also look up variants of the blob which have the same
// uncompressed digest.
func (mem *memoryCache) CandidateLocations(transport types.ImageTransport, scope types.BICTransportScope, primaryDigest digest.Digest, canSubstitute bool) []types.BICReplacementCandidate {
	res := []candidateWithTime{}
	res = mem.appendReplacementCandidates(res, transport, scope, primaryDigest)
	var uncompressedDigest digest.Digest // = ""
	if canSubstitute {
		if uncompressedDigest = mem.UncompressedDigest(primaryDigest); uncompressedDigest != "" {
			otherDigests := mem.digestsByUncompressed[uncompressedDigest] // nil if not present in the map
			for d := range otherDigests {
				if d != primaryDigest && d != uncompressedDigest {
					res = mem.appendReplacementCandidates(res, transport, scope, d)
				}
			}
			if uncompressedDigest != primaryDigest {
				res = mem.appendReplacementCandidates(res, transport, scope, uncompressedDigest)
			}
		}
	}
	return destructivelyPrioritizeReplacementCandidates(res, primaryDigest, uncompressedDigest)
}
