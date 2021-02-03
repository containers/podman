// Package prioritize provides utilities for prioritizing locations in
// types.BlobInfoCache.CandidateLocations.
package prioritize

import (
	"sort"
	"time"

	"github.com/containers/image/v5/internal/blobinfocache"
	"github.com/opencontainers/go-digest"
)

// replacementAttempts is the number of blob replacement candidates returned by destructivelyPrioritizeReplacementCandidates,
// and therefore ultimately by types.BlobInfoCache.CandidateLocations.
// This is a heuristic/guess, and could well use a different value.
const replacementAttempts = 5

// CandidateWithTime is the input to types.BICReplacementCandidate prioritization.
type CandidateWithTime struct {
	Candidate blobinfocache.BICReplacementCandidate2 // The replacement candidate
	LastSeen  time.Time                              // Time the candidate was last known to exist (either read or written)
}

// candidateSortState is a local state implementing sort.Interface on candidates to prioritize,
// along with the specially-treated digest values for the implementation of sort.Interface.Less
type candidateSortState struct {
	cs                 []CandidateWithTime // The entries to sort
	primaryDigest      digest.Digest       // The digest the user actually asked for
	uncompressedDigest digest.Digest       // The uncompressed digest corresponding to primaryDigest. May be "", or even equal to primaryDigest
}

func (css *candidateSortState) Len() int {
	return len(css.cs)
}

func (css *candidateSortState) Less(i, j int) bool {
	xi := css.cs[i]
	xj := css.cs[j]

	// primaryDigest entries come first, more recent first.
	// uncompressedDigest entries, if uncompressedDigest is set and != primaryDigest, come last, more recent entry first.
	// Other digest values are primarily sorted by time (more recent first), secondarily by digest (to provide a deterministic order)

	// First, deal with the primaryDigest/uncompressedDigest cases:
	if xi.Candidate.Digest != xj.Candidate.Digest {
		// - The two digests are different, and one (or both) of the digests is primaryDigest or uncompressedDigest: time does not matter
		if xi.Candidate.Digest == css.primaryDigest {
			return true
		}
		if xj.Candidate.Digest == css.primaryDigest {
			return false
		}
		if css.uncompressedDigest != "" {
			if xi.Candidate.Digest == css.uncompressedDigest {
				return false
			}
			if xj.Candidate.Digest == css.uncompressedDigest {
				return true
			}
		}
	} else { // xi.Candidate.Digest == xj.Candidate.Digest
		// The two digests are the same, and are either primaryDigest or uncompressedDigest: order by time
		if xi.Candidate.Digest == css.primaryDigest || (css.uncompressedDigest != "" && xi.Candidate.Digest == css.uncompressedDigest) {
			return xi.LastSeen.After(xj.LastSeen)
		}
	}

	// Neither of the digests are primaryDigest/uncompressedDigest:
	if !xi.LastSeen.Equal(xj.LastSeen) { // Order primarily by time
		return xi.LastSeen.After(xj.LastSeen)
	}
	// Fall back to digest, if timestamps end up _exactly_ the same (how?!)
	return xi.Candidate.Digest < xj.Candidate.Digest
}

func (css *candidateSortState) Swap(i, j int) {
	css.cs[i], css.cs[j] = css.cs[j], css.cs[i]
}

// destructivelyPrioritizeReplacementCandidatesWithMax is destructivelyPrioritizeReplacementCandidates with a parameter for the
// number of entries to limit, only to make testing simpler.
func destructivelyPrioritizeReplacementCandidatesWithMax(cs []CandidateWithTime, primaryDigest, uncompressedDigest digest.Digest, maxCandidates int) []blobinfocache.BICReplacementCandidate2 {
	// We don't need to use sort.Stable() because nanosecond timestamps are (presumably?) unique, so no two elements should
	// compare equal.
	sort.Sort(&candidateSortState{
		cs:                 cs,
		primaryDigest:      primaryDigest,
		uncompressedDigest: uncompressedDigest,
	})

	resLength := len(cs)
	if resLength > maxCandidates {
		resLength = maxCandidates
	}
	res := make([]blobinfocache.BICReplacementCandidate2, resLength)
	for i := range res {
		res[i] = cs[i].Candidate
	}
	return res
}

// DestructivelyPrioritizeReplacementCandidates consumes AND DESTROYS an array of possible replacement candidates with their last known existence times,
// the primary digest the user actually asked for, and the corresponding uncompressed digest (if known, possibly equal to the primary digest),
// and returns an appropriately prioritized and/or trimmed result suitable for a return value from types.BlobInfoCache.CandidateLocations.
//
// WARNING: The array of candidates is destructively modified. (The implementation of this function could of course
// make a copy, but all CandidateLocations implementations build the slice of candidates only for the single purpose of calling this function anyway.)
func DestructivelyPrioritizeReplacementCandidates(cs []CandidateWithTime, primaryDigest, uncompressedDigest digest.Digest) []blobinfocache.BICReplacementCandidate2 {
	return destructivelyPrioritizeReplacementCandidatesWithMax(cs, primaryDigest, uncompressedDigest, replacementAttempts)
}
