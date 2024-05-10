// Package prioritize provides utilities for filtering and prioritizing locations in
// types.BlobInfoCache.CandidateLocations.
package prioritize

import (
	"cmp"
	"slices"
	"time"

	"github.com/containers/image/v5/internal/blobinfocache"
	"github.com/containers/image/v5/internal/manifest"
	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

// replacementAttempts is the number of blob replacement candidates with known location returned by destructivelyPrioritizeReplacementCandidates,
// and therefore ultimately by types.BlobInfoCache.CandidateLocations.
// This is a heuristic/guess, and could well use a different value.
const replacementAttempts = 5

// replacementUnknownLocationAttempts is the number of blob replacement candidates with unknown Location returned by destructivelyPrioritizeReplacementCandidates,
// and therefore ultimately by blobinfocache.BlobInfoCache2.CandidateLocations2.
// This is a heuristic/guess, and could well use a different value.
const replacementUnknownLocationAttempts = 2

// CandidateCompression returns (true, compressionOp, compressionAlgo) if a blob
// with compressionName (which can be Uncompressed or UnknownCompression) is acceptable for a CandidateLocations* call with v2Options.
//
// v2Options can be set to nil if the call is CandidateLocations (i.e. compression is not required to be known);
// if not nil, the call is assumed to be CandidateLocations2.
//
// The (compressionOp, compressionAlgo) values are suitable for BICReplacementCandidate2
func CandidateCompression(v2Options *blobinfocache.CandidateLocations2Options, digest digest.Digest, compressorName string) (bool, types.LayerCompression, *compression.Algorithm) {
	if v2Options == nil {
		return true, types.PreserveOriginal, nil // Anything goes. The (compressionOp, compressionAlgo) values are not used.
	}

	var op types.LayerCompression
	var algo *compression.Algorithm
	switch compressorName {
	case blobinfocache.Uncompressed:
		op = types.Decompress
		algo = nil
	case blobinfocache.UnknownCompression:
		logrus.Debugf("Ignoring BlobInfoCache record of digest %q with unknown compression", digest.String())
		return false, types.PreserveOriginal, nil // Not allowed with CandidateLocations2
	default:
		op = types.Compress
		algo_, err := compression.AlgorithmByName(compressorName)
		if err != nil {
			logrus.Debugf("Ignoring BlobInfoCache record of digest %q with unrecognized compression %q: %v",
				digest.String(), compressorName, err)
			return false, types.PreserveOriginal, nil // The BICReplacementCandidate2.CompressionAlgorithm field is required
		}
		algo = &algo_
	}
	if !manifest.CandidateCompressionMatchesReuseConditions(manifest.ReuseConditions{
		PossibleManifestFormats: v2Options.PossibleManifestFormats,
		RequiredCompression:     v2Options.RequiredCompression,
	}, algo) {
		requiredCompresssion := "nil"
		if v2Options.RequiredCompression != nil {
			requiredCompresssion = v2Options.RequiredCompression.Name()
		}
		logrus.Debugf("Ignoring BlobInfoCache record of digest %q, compression %q does not match required %s or MIME types %#v",
			digest.String(), compressorName, requiredCompresssion, v2Options.PossibleManifestFormats)
		return false, types.PreserveOriginal, nil
	}

	return true, op, algo
}

// CandidateWithTime is the input to types.BICReplacementCandidate prioritization.
type CandidateWithTime struct {
	Candidate blobinfocache.BICReplacementCandidate2 // The replacement candidate
	LastSeen  time.Time                              // Time the candidate was last known to exist (either read or written) (not set for Candidate.UnknownLocation)
}

// candidateSortState is a closure for a comparison used by slices.SortFunc on candidates to prioritize,
// along with the specially-treated digest values relevant to the ordering.
type candidateSortState struct {
	primaryDigest      digest.Digest // The digest the user actually asked for
	uncompressedDigest digest.Digest // The uncompressed digest corresponding to primaryDigest. May be "", or even equal to primaryDigest
}

func (css *candidateSortState) compare(xi, xj CandidateWithTime) int {
	// primaryDigest entries come first, more recent first.
	// uncompressedDigest entries, if uncompressedDigest is set and != primaryDigest, come last, more recent entry first.
	// Other digest values are primarily sorted by time (more recent first), secondarily by digest (to provide a deterministic order)

	// First, deal with the primaryDigest/uncompressedDigest cases:
	if xi.Candidate.Digest != xj.Candidate.Digest {
		// - The two digests are different, and one (or both) of the digests is primaryDigest or uncompressedDigest: time does not matter
		if xi.Candidate.Digest == css.primaryDigest {
			return -1
		}
		if xj.Candidate.Digest == css.primaryDigest {
			return 1
		}
		if css.uncompressedDigest != "" {
			if xi.Candidate.Digest == css.uncompressedDigest {
				return 1
			}
			if xj.Candidate.Digest == css.uncompressedDigest {
				return -1
			}
		}
	} else { // xi.Candidate.Digest == xj.Candidate.Digest
		// The two digests are the same, and are either primaryDigest or uncompressedDigest: order by time
		if xi.Candidate.Digest == css.primaryDigest || (css.uncompressedDigest != "" && xi.Candidate.Digest == css.uncompressedDigest) {
			return -xi.LastSeen.Compare(xj.LastSeen)
		}
	}

	// Neither of the digests are primaryDigest/uncompressedDigest:
	if cmp := xi.LastSeen.Compare(xj.LastSeen); cmp != 0 { // Order primarily by time
		return -cmp
	}
	// Fall back to digest, if timestamps end up _exactly_ the same (how?!)
	return cmp.Compare(xi.Candidate.Digest, xj.Candidate.Digest)
}

// destructivelyPrioritizeReplacementCandidatesWithMax is destructivelyPrioritizeReplacementCandidates with parameters for the
// number of entries to limit for known and unknown location separately, only to make testing simpler.
// TODO: following function is not destructive any more in the nature instead prioritized result is actually copies of the original
// candidate set, so In future we might wanna re-name this public API and remove the destructive prefix.
func destructivelyPrioritizeReplacementCandidatesWithMax(cs []CandidateWithTime, primaryDigest, uncompressedDigest digest.Digest, totalLimit int, noLocationLimit int) []blobinfocache.BICReplacementCandidate2 {
	// split unknown candidates and known candidates
	// and limit them separately.
	var knownLocationCandidates []CandidateWithTime
	var unknownLocationCandidates []CandidateWithTime
	// We don't need to use sort.Stable() because nanosecond timestamps are (presumably?) unique, so no two elements should
	// compare equal.
	slices.SortFunc(cs, (&candidateSortState{
		primaryDigest:      primaryDigest,
		uncompressedDigest: uncompressedDigest,
	}).compare)
	for _, candidate := range cs {
		if candidate.Candidate.UnknownLocation {
			unknownLocationCandidates = append(unknownLocationCandidates, candidate)
		} else {
			knownLocationCandidates = append(knownLocationCandidates, candidate)
		}
	}

	knownLocationCandidatesUsed := min(len(knownLocationCandidates), totalLimit)
	remainingCapacity := totalLimit - knownLocationCandidatesUsed
	unknownLocationCandidatesUsed := min(noLocationLimit, remainingCapacity, len(unknownLocationCandidates))
	res := make([]blobinfocache.BICReplacementCandidate2, knownLocationCandidatesUsed)
	for i := 0; i < knownLocationCandidatesUsed; i++ {
		res[i] = knownLocationCandidates[i].Candidate
	}
	// If candidates with unknown location are found, lets add them to final list
	for i := 0; i < unknownLocationCandidatesUsed; i++ {
		res = append(res, unknownLocationCandidates[i].Candidate)
	}
	return res
}

// DestructivelyPrioritizeReplacementCandidates consumes AND DESTROYS an array of possible replacement candidates with their last known existence times,
// the primary digest the user actually asked for, the corresponding uncompressed digest (if known, possibly equal to the primary digest) returns an
// appropriately prioritized and/or trimmed result suitable for a return value from types.BlobInfoCache.CandidateLocations.
//
// WARNING: The array of candidates is destructively modified. (The implementation of this function could of course
// make a copy, but all CandidateLocations implementations build the slice of candidates only for the single purpose of calling this function anyway.)
func DestructivelyPrioritizeReplacementCandidates(cs []CandidateWithTime, primaryDigest, uncompressedDigest digest.Digest) []blobinfocache.BICReplacementCandidate2 {
	return destructivelyPrioritizeReplacementCandidatesWithMax(cs, primaryDigest, uncompressedDigest, replacementAttempts, replacementUnknownLocationAttempts)
}
