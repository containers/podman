// Copyright 2020 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package classifier

import (
	"fmt"
	"hash/crc32"
	"math"
	"sort"

	"github.com/davecgh/go-spew/spew"
)

// searchSet is a set of q-grams that have hashes associated with them,
// making it fast to search for potential matches.
type searchSet struct {
	// Tokens is a tokenized list of the original input string.
	Tokens []indexedToken
	// Hashes is a map of checksums to a range of tokens.
	Hashes hash
	// Checksums is a list of checksums ordered from longest range to
	// shortest.
	Checksums []uint32
	// ChecksumRanges are the token ranges for the above checksums.
	ChecksumRanges tokenRanges
	origin         string // A debugging identifier to label what this searchset is associated with

	nodes []*node
	q     int // The length of q-grams in this searchset.
}

// node consists of a range of tokens along with the checksum for those tokens.
type node struct {
	checksum uint32
	tokens   *tokenRange
}

func (n *node) String() string {
	return fmt.Sprintf("[%d:%d]", n.tokens.Start, n.tokens.End)
}

// newSearchSet creates a new searchSet object. A searchset generates all
// possible q-grams of tokens. These q-grams of tokens can be correlated to
// determine where a section of text from one source may appear in another
// source.
func newSearchSet(s *indexedDocument, q int) *searchSet {
	// Start generating hash values for all q-grams within the text.
	h := make(hash)
	if len(s.Tokens) < q {
		// We can't have a smaller q than the number of tokens.
		q = len(s.Tokens)
	}
	checksums, tokenRanges := generateHashes(h, q, s.Tokens, s.dict)
	sset := &searchSet{
		Tokens:         s.Tokens,
		Hashes:         h,
		Checksums:      checksums,
		ChecksumRanges: tokenRanges,
		q:              q,
	}
	sset.generateNodeList()
	return sset
}

// tokenRange indicates the range of tokens that map to a particular checksum.
type tokenRange struct {
	Start int
	End   int
}

func (t *tokenRange) String() string {
	return fmt.Sprintf("[%v, %v)", t.Start, t.End)
}

// tokenRanges is a sortable type of a slice of TokenRange.
type tokenRanges []*tokenRange

// generateHashes computes a hash using CRC-32 for each q-gram encountered in the provided tokens.
func generateHashes(h hash, q int, toks []indexedToken, dict *dictionary) ([]uint32, tokenRanges) {
	if q == 0 {
		return nil, nil
	}
	var css []uint32
	var tr tokenRanges
	crc := crc32.NewIEEE()
	for offset := 0; offset+q <= len(toks); offset++ {
		crc.Reset()
		for i := 0; i < q; i++ {
			crc.Write([]byte(dict.getWord(toks[offset+i].ID)))
			crc.Write([]byte{' '})
		}
		cs := crc.Sum32()
		css = append(css, cs)
		tr = append(tr, &tokenRange{offset, offset + q})
		h.add(cs, offset, offset+q)
	}

	return css, tr
}

// generateNodeList creates a node list out of the search set.
func (s *searchSet) generateNodeList() {
	if len(s.Tokens) == 0 {
		return
	}

	for i := 0; i < len(s.Checksums); i++ {
		s.nodes = append(s.nodes, &node{
			checksum: s.Checksums[i],
			tokens:   s.ChecksumRanges[i],
		})
	}
}

// matchRange is the range within the source text that is a match to the range
// in the target text.
type matchRange struct {
	// Offsets into the source tokens.
	SrcStart, SrcEnd int
	// Offsets into the target tokens.
	TargetStart, TargetEnd int
	// TokensClaimed tracks the number of positively matched tokens in this
	// range.  For initially created matchRanges, this is equal to the extent of
	// the range.  However, as matchRanges get merged together and error is
	// potentially introduced, this tracks the precise number of tokens that
	// exist in the range.
	TokensClaimed int
}

// in returns true if the start and end are enclosed in the match range.
func (m *matchRange) in(other *matchRange) bool {
	return m.TargetStart >= other.TargetStart && m.TargetEnd <= other.TargetEnd
}

func (m *matchRange) String() string {
	return fmt.Sprintf("S: [%v, %v)-> T: [%v, %v) => %v [%v]", m.SrcStart, m.SrcEnd, m.TargetStart, m.TargetEnd, m.TargetStart-m.SrcStart, m.TokensClaimed)
}

// matchRanges is a list of "matchRange"s. The ranges are monotonically
// increasing in value and indicate a single potential occurrence of the source
// text in the target text. They are sorted to support greedy matching with the
// longest runs of q-grams appearing first in the sort.
type matchRanges []*matchRange

func (m matchRanges) Len() int      { return len(m) }
func (m matchRanges) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m matchRanges) Less(i, j int) bool {
	if m[i].TokensClaimed != m[j].TokensClaimed {
		return m[i].TokensClaimed > m[j].TokensClaimed
	}

	if m[i].TargetStart != m[j].TargetStart {
		return m[i].TargetStart < m[j].TargetStart
	}
	return m[i].SrcStart < m[j].SrcStart
}

// findPotentialMatches returns the ranges in the target (unknown) text that
// are best potential matches to the source (known) text.
func (c *Classifier) findPotentialMatches(src, target *searchSet, confidence float64) matchRanges {
	matchedRanges := c.getMatchedRanges(src, target, confidence, src.q)
	if c.tc.traceSearchset(src.origin) {
		c.tc.trace("matchedRanges = %s", spew.Sdump(matchedRanges))
	}
	if len(matchedRanges) == 0 {
		return nil
	}

	// After computing all potential matches, we only output ranges that contain
	// enough tokens to clear the confidence threshold. As noted, this number can
	// be too high, yielding false positives, but cannot yield false negatives.
	threshold := int(confidence * float64(len(src.Tokens)))

	for i, m := range matchedRanges {
		if m.TokensClaimed < threshold {
			matchedRanges = matchedRanges[:i]
			break
		}
	}

	return matchedRanges
}

// fuseRanges analyzes the source matches, attempting to combine hits without
// errors into larger hits with tolerable amounts of error to produce matches
// that contain enough tokens to be considered for exact matching against a a
// target document. This routine intentionally does not accurately track error
// contributions from merging runs, trading false positives (but not false
// negatives), for faster performance.
func (c *Classifier) fuseRanges(origin string, matched matchRanges, confidence float64, size int, runs []matchRange, targetSize int) matchRanges {
	var claimed matchRanges
	errorMargin := int(math.Round(float64(size) * (1.0 - confidence)))

	filter := make([]bool, targetSize)
	for _, m := range runs {
		for i := m.SrcStart; i < m.SrcEnd; i++ {
			filter[i] = true
		}
	}

	filterDrops := 0
	filterPasses := 0

	// For each hit detected, compare it against all other previous hits to see if it can be part of match
	// or represents a group that is eligible for matching and having other hits contribute to it.
	for i, m := range matched {
		off := m.TargetStart - m.SrcStart

		// If the offset is negative, but within error margins, we associate it
		// with the first index to see if it could contribute to a run. If the
		// absolute offset is larger than the error margin, it can't possibly
		// contribute and will be dropped. This puts more potential error into the zero
		// index, but that just slightly increases the rate of false positives. In
		// practice, this would only be an issue if there are major substrings of a
		// source in a target that aren't part of a real hit. We see many small
		// references (the name of a license) but not large chunks of the license.
		if off < 0 {
			if -off <= errorMargin {
				off = 0
			} else {
				continue
			}
		}

		// If the filter is false, there was not sufficient token density in that
		// part of the target document for a viable match, so this match is a
		// spurious hit and can be discarded.
		if !filter[off] {
			filterDrops++
			continue
		}

		filterPasses++
		unclaimed := true

		for _, c := range claimed {
			moff := m.TargetStart - m.SrcStart
			coff := c.TargetStart - c.SrcStart
			sampleError := int(math.Round(math.Abs(float64(moff - coff))))
			withinError := sampleError < errorMargin

			// The contribution needs to add more value than accumulated error. This prevents
			// against spurious matches of a reference to a license incorrectly overextending the
			// match range.
			if withinError && m.TokensClaimed > int(sampleError) {
				if m.in(c) {
					// This can cause too many tokens to be claimed, but that's fine since we want to avoid
					// undercounting and missing content.
					c.TokensClaimed += m.TokensClaimed
					unclaimed = false
				} else {
					// See if the claims can be merged. If the error tolerances allow for it,
					// we merge the claims and the ranges. This doesn't accumulate error
					// accurately so it's possible that repeated merges can introduce too
					// much error to actually make a match, but we won't get false
					// negatives from this approach.  The error tolerances allow for a
					// merge, but we only want to merge if it increases the range of
					// tokens being covered. If this match is already handled by an
					// existing (stronger by definition) claim, we don't merge this one,
					// but treat it as a new claim. This allows for the case where a
					// highly fragmented text will be matched by a long series of low
					// score matches.
					if m.TargetStart < c.TargetStart && m.SrcStart < c.SrcStart {
						c.TargetStart = m.TargetStart
						c.SrcStart = m.SrcStart
						c.TokensClaimed += m.TokensClaimed
						unclaimed = false
					} else if m.TargetEnd > c.TargetEnd && m.SrcEnd > c.SrcEnd {
						c.TargetEnd = m.TargetEnd
						c.SrcEnd = m.SrcEnd
						c.TokensClaimed += m.TokensClaimed
						unclaimed = false
					}
					// This claim does not extend any existing block, and it may be claimed in its own
					// right.
				}
			}
			if !unclaimed {
				break
			}
		}
		// Only create a claim if this claim is likely relevant. If we had some higher quality hits,
		// it's likely this is spurious noise. If we haven't had any significantly better hits, we'll keep
		// this around.
		if unclaimed && m.TokensClaimed*10 > matched[0].TokensClaimed {
			claimed = append(claimed, m)
		}
		if c.tc.traceSearchset(origin) {
			c.tc.trace("after %d ranges, claimed is %s", i, spew.Sdump(claimed))
		}
	}
	sort.Sort(claimed)
	if c.tc.traceSearchset(origin) {
		c.tc.trace("filterPasses = %+v", filterPasses)
		c.tc.trace("filterDrops = %+v", filterDrops)
		c.tc.trace("claimed = %s", spew.Sdump(claimed))
	}
	return claimed
}

// getMatchedRanges finds the ranges in the target text that match the source
// text. The ranges returned are ordered from the entries with the most matched
// tokens to the least.
func (c *Classifier) getMatchedRanges(src, target *searchSet, confidence float64, q int) matchRanges {
	shouldTrace := c.tc.traceSearchset(src.origin)

	if shouldTrace {
		c.tc.trace("src.origin = %+v", src.origin)
	}
	// Assemble a list of all the matched q-grams without any consideration to
	// error tolerances.
	matched := targetMatchedRanges(src, target)
	if shouldTrace {
		c.tc.trace("matched = %s", spew.Sdump(matched))
	}
	if len(matched) == 0 {
		return nil
	}

	// Perform neighborhood matching to figure out which clusters of q-grams have
	// sufficient likelihood to be a potential match to the source. For an error
	// confidence threshold of X, we require that a sequence of N target tokens
	// must contain N*X (X <=1.0) ordered source tokens in order to be a viable
	// match.
	//
	// It's much easier to compute potential ranges in the target if we disregard
	// proper ordering of source tokens initially, and see which source q-grams
	// appear in sufficient quantities to be a potential match. We can then
	// disregard any q-gram that falls outside of that range. This helps
	// significantly since processing token matches is an N^2 (or worse)
	// operation, so reducing N is a big win.

	runs := c.detectRuns(src.origin, matched, len(target.Tokens), len(src.Tokens), confidence, q)

	if shouldTrace {
		c.tc.trace("runs = %d: %s", len(runs), spew.Sdump(runs))
	}

	// If there are no target runs of source tokens, we're done.
	if len(runs) == 0 {
		return nil
	}

	// Using the runs as a filter to ignore irrelevant matches, fuse the source
	// match ranges into larger matches (with possible errors) to see if we can
	// produce large enough runs that pass the confidence threshold.

	fr := c.fuseRanges(src.origin, matched, confidence, len(src.Tokens), runs, len(target.Tokens))
	if shouldTrace {
		c.tc.trace("fr = %s", spew.Sdump(fr))
	}
	return fr
}

func (c *Classifier) detectRuns(origin string, matched matchRanges, targetLength, subsetLength int, threshold float64, q int) []matchRange {
	shouldTrace := c.tc.traceSearchset(origin)
	hits := make([]bool, targetLength)
	for _, m := range matched {
		for idx := m.TargetStart; idx < m.TargetEnd; idx++ {
			hits[idx] = true
		}
	}

	if len(hits) == 0 {
		return nil
	}
	var out []int

	total := 0
	target := int(float64(subsetLength) * threshold)
	if shouldTrace {
		c.tc.trace("target = %+v", target)
		c.tc.trace("targetLength = %+v", targetLength)
		c.tc.trace("subsetLength = %+v", subsetLength)
	}

	// If we don't have at least 1 subset (i.e. the target is shorter than the
	// source) just analyze what we have.
	if len(hits) < subsetLength {
		if shouldTrace {
			c.tc.trace("trimmed search length from %d to %d", subsetLength, len(hits))
		}
		subsetLength = len(hits)
	}
	// Initialize our sliding window value.
	for i := 0; i < subsetLength; i++ {
		if hits[i] {
			total++
		}
	}

	if total >= target {
		out = append(out, 0)
	}

	// Now move through the window adjusting the total by subtracting out the
	// last bit and adding in the new bit.
	for i := 1; i < len(hits); i++ {
		if hits[i-1] {
			total--
		}
		end := i + subsetLength - 1
		if end < len(hits) && hits[i+subsetLength-1] {
			total++
		}
		if total >= target {
			out = append(out, i)
		}
	}
	if len(out) == 0 {
		return nil
	}

	final := []matchRange{
		{
			SrcStart: out[0],
			SrcEnd:   out[0] + q,
		},
	}
	for i := 1; i < len(out); i++ {
		if out[i] != 1+out[i-1] {
			final = append(final, matchRange{
				SrcStart: out[i],
				SrcEnd:   out[i] + q,
			})
		} else {
			final[len(final)-1].SrcEnd = out[i] + q
		}
	}

	return final
}

func targetMatchedRanges(src, target *searchSet) matchRanges {
	offsetMappings := make(map[int][]*matchRange)

	var matched matchRanges
	for _, tgtNode := range target.nodes {
		sr, ok := src.Hashes[tgtNode.checksum]
		if !ok {
			continue
		}

		tv := tgtNode.tokens
		for _, sv := range sr {
			offset := tv.Start - sv.Start
			if om, ok := offsetMappings[offset]; ok {
				// See if this extends the most recent existing mapping
				lastIdx := len(om) - 1
				if om[lastIdx].TargetEnd == tv.End-1 {
					// This new value extends. Update the value in place
					om[lastIdx].SrcEnd = sv.End
					om[lastIdx].TargetEnd = tv.End
					continue
				}
			}
			offsetMappings[offset] = append(offsetMappings[offset], &matchRange{
				SrcStart:    sv.Start,
				SrcEnd:      sv.End,
				TargetStart: tv.Start,
				TargetEnd:   tv.End,
			})
		}
	}

	// Compute the number of tokens claimed in each run and flatten into a single slice.
	for _, mr := range offsetMappings {
		for _, m := range mr {
			m.TokensClaimed = m.TargetEnd - m.TargetStart
		}
		matched = append(matched, mr...)
	}
	sort.Sort(matched)
	return matched
}

type hash map[uint32]tokenRanges

func (h hash) add(checksum uint32, start, end int) {
	h[checksum] = append(h[checksum], &tokenRange{Start: start, End: end})
}
