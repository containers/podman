package blobinfocache

import (
	"sort"
	"time"

	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
)

// replacementAttempts is the number of blob replacement candidates returned by destructivelyPrioritizeReplacementCandidates,
// and therefore ultimately by types.BlobInfoCache.CandidateLocations.
// This is a heuristic/guess, and could well use a different value.
const replacementAttempts = 5

// candidateWithTime is the input to types.BICReplacementCandidate prioritization.
type candidateWithTime struct {
	candidate types.BICReplacementCandidate // The replacement candidate
	lastSeen  time.Time                     // Time the candidate was last known to exist (either read or written)
}

// candidateSortState is a local state implementing sort.Interface on candidates to prioritize,
// along with the specially-treated digest values for the implementation of sort.Interface.Less
type candidateSortState struct {
	cs                 []candidateWithTime // The entries to sort
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
	if xi.candidate.Digest != xj.candidate.Digest {
		// - The two digests are different, and one (or both) of the digests is primaryDigest or uncompressedDigest: time does not matter
		if xi.candidate.Digest == css.primaryDigest {
			return true
		}
		if xj.candidate.Digest == css.primaryDigest {
			return false
		}
		if css.uncompressedDigest != "" {
			if xi.candidate.Digest == css.uncompressedDigest {
				return false
			}
			if xj.candidate.Digest == css.uncompressedDigest {
				return true
			}
		}
	} else { // xi.candidate.Digest == xj.candidate.Digest
		// The two digests are the same, and are either primaryDigest or uncompressedDigest: order by time
		if xi.candidate.Digest == css.primaryDigest || (css.uncompressedDigest != "" && xi.candidate.Digest == css.uncompressedDigest) {
			return xi.lastSeen.After(xj.lastSeen)
		}
	}

	// Neither of the digests are primaryDigest/uncompressedDigest:
	if !xi.lastSeen.Equal(xj.lastSeen) { // Order primarily by time
		return xi.lastSeen.After(xj.lastSeen)
	}
	// Fall back to digest, if timestamps end up _exactly_ the same (how?!)
	return xi.candidate.Digest < xj.candidate.Digest
}

func (css *candidateSortState) Swap(i, j int) {
	css.cs[i], css.cs[j] = css.cs[j], css.cs[i]
}

// destructivelyPrioritizeReplacementCandidatesWithMax is destructivelyPrioritizeReplacementCandidates with a parameter for the
// number of entries to limit, only to make testing simpler.
func destructivelyPrioritizeReplacementCandidatesWithMax(cs []candidateWithTime, primaryDigest, uncompressedDigest digest.Digest, maxCandidates int) []types.BICReplacementCandidate {
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
	res := make([]types.BICReplacementCandidate, resLength)
	for i := range res {
		res[i] = cs[i].candidate
	}
	return res
}

// destructivelyPrioritizeReplacementCandidates consumes AND DESTROYS an array of possible replacement candidates with their last known existence times,
// the primary digest the user actually asked for, and the corresponding uncompressed digest (if known, possibly equal to the primary digest),
// and returns an appropriately prioritized and/or trimmed result suitable for a return value from types.BlobInfoCache.CandidateLocations.
//
// WARNING: The array of candidates is destructively modified. (The implementation of this function could of course
// make a copy, but all CandidateLocations implementations build the slice of candidates only for the single purpose of calling this function anyway.)
func destructivelyPrioritizeReplacementCandidates(cs []candidateWithTime, primaryDigest, uncompressedDigest digest.Digest) []types.BICReplacementCandidate {
	return destructivelyPrioritizeReplacementCandidatesWithMax(cs, primaryDigest, uncompressedDigest, replacementAttempts)
}
