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
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Match is the information about a single instance of a detected match.
type Match struct {
	Name            string
	Confidence      float64
	MatchType       string
	StartLine       int
	EndLine         int
	StartTokenIndex int
	EndTokenIndex   int
}

// Matches is a sortable slice of Match.
type Matches []*Match

// Swap two elements of Matches.
func (d Matches) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d Matches) Len() int      { return len(d) }
func (d Matches) Less(i, j int) bool {
	di, dj := d[i], d[j]
	// Return matches ordered by confidence
	if di.Confidence != dj.Confidence {
		return di.Confidence > dj.Confidence
	}
	// Licenses of same confidence are ordered by their appearance
	if di.StartTokenIndex != dj.StartTokenIndex {
		return di.StartTokenIndex < dj.StartTokenIndex
	}
	// Should never get here, but tiebreak based on the larger license.
	return di.EndTokenIndex > dj.EndTokenIndex
}

// Match reports instances of the supplied content in the corpus.
func (c *Classifier) match(in []byte) Matches {
	id := c.createTargetIndexedDocument(in)

	firstPass := make(map[string]*indexedDocument)
	for l, d := range c.docs {
		sim := id.tokenSimilarity(d)
		if sim >= c.threshold {
			firstPass[l] = d
		}
	}

	if len(firstPass) == 0 {
		return nil
	}

	// Perform the expensive work of generating a searchset to look for token runs.
	id.generateSearchSet(c.q)

	var candidates Matches
	for l, d := range firstPass {
		matches := c.findPotentialMatches(d.s, id.s, c.threshold)
		for _, m := range matches {
			startIndex := m.TargetStart
			endIndex := m.TargetEnd
			conf, startOffset, endOffset := c.score(l, id, d, startIndex, endIndex)
			if conf >= c.threshold && (endIndex-startIndex-startOffset-endOffset) > 0 {
				candidates = append(candidates, &Match{
					Name:            LicenseName(l),
					MatchType:       detectionType(l),
					Confidence:      conf,
					StartLine:       id.Tokens[startIndex+startOffset].Line,
					EndLine:         id.Tokens[endIndex-endOffset-1].Line,
					StartTokenIndex: id.Tokens[startIndex+startOffset].Index,
					EndTokenIndex:   id.Tokens[endIndex-endOffset-1].Index,
				})
			}

		}
	}
	sort.Sort(candidates)
	retain := make([]bool, len(candidates))
	for i, c := range candidates {
		// Filter out overlapping licenses based primarily on confidence. Since
		// the candidates slice is ordered by confidence, we look for overlaps and
		// decide if we retain the record c.

		// For each candidate, only add it to the report unless we have a
		// higher-quality hit that contains these lines. In the case of two
		// licenses having overlap, we consider 'token density' to break ties. If a
		// less confident match of a larger license has more matching tokens than a
		// perfect match of a smaller license, we want to keep that. This handles
		// licenses that include another license as a subtext. NPL contains MPL
		// as a concrete example.

		keep := true
		proposals := make(map[int]bool)
		for j, o := range candidates {
			if j == i {
				break
			}
			// Make sure to only check containment on licenses that are still in consideration at this point.
			if contains(c, o) && retain[j] {
				// The license here can override a previous detection, but that isn't sufficient to be kept
				// on its own. Consider the licenses Xnet, MPL-1.1 and NPL-1.1 in a file that just has MPL-1.1.
				// The confidence rating on NPL-1.1 will cause Xnet to not be retained, which is correct, but it
				// shouldn't be retained if the token confidence for MPL is higher than NPL since the NPL-specific
				// bits are missing.

				ctoks := float64(c.EndTokenIndex - c.StartTokenIndex)
				otoks := float64(o.EndTokenIndex - o.StartTokenIndex)
				cconf := ctoks * c.Confidence
				oconf := otoks * o.Confidence

				// If the two licenses are exactly the same confidence, that means we
				// have an ambiguous detect and should retain both, so the caller can
				// see and resolve the situation.
				if cconf > oconf {
					proposals[j] = false
				} else if oconf > cconf {
					keep = false
				}
			} else if overlaps(c, o) && retain[j] {
				keep = false
			}

		}
		if keep {
			retain[i] = true
			for p, v := range proposals {
				retain[p] = v
			}
		}
	}

	var out Matches
	for i, keep := range retain {
		if keep {
			out = append(out, candidates[i])
		}
	}
	return out
}

// Classifier provides methods for identifying open source licenses in text
// content.
type Classifier struct {
	tc        *TraceConfiguration
	dict      *dictionary
	docs      map[string]*indexedDocument
	threshold float64
	q         int // The value of q for q-grams in this corpus
}

// NewClassifier creates a classifier with an empty corpus.
func NewClassifier(threshold float64) *Classifier {
	classifier := &Classifier{
		tc:        new(TraceConfiguration),
		dict:      newDictionary(),
		docs:      make(map[string]*indexedDocument),
		threshold: threshold,
		q:         computeQ(threshold),
	}
	return classifier
}

// LoadLicenses adds the contents of the supplied directory to the corpus of the
// classifier.
func (c *Classifier) LoadLicenses(dir string) error {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !strings.HasSuffix(path, "txt") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return err
	}

	for _, f := range files {
		_, name := path.Split(f)
		name = strings.Replace(name, ".txt", "", 1)
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return err
		}

		content := trimExtraneousTrailingText(string(b))
		c.AddContent(name, []byte(content))
	}
	return nil
}

// SetTraceConfiguration installs a tracing configuration for the classifier.
func (c *Classifier) SetTraceConfiguration(in *TraceConfiguration) {
	c.tc = in
	c.tc.init()
}

// Match finds matches within an unknown text. This will not modify the contents
// of the supplied byte slice.
func (c *Classifier) Match(in []byte) Matches {
	return c.match(in)
}

// MatchFrom finds matches within the read content.
func (c *Classifier) MatchFrom(in io.Reader) (Matches, error) {
	b, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("classifier couldn't read: %w", err)
	}
	return c.Match(b), nil
}

func detectionType(in string) string {
	if strings.Index(in, ".header") != -1 {
		return "Header"
	}
	return "License"
}

// LicenseName produces the output name for a license, removing the internal structure
// of the filename in use.
func LicenseName(in string) string {
	out := in
	if idx := strings.Index(in, ".txt"); idx != -1 {
		out = out[0:idx]
	}
	if idx := strings.Index(in, "_"); idx != -1 {
		out = out[0:idx]
	}
	if idx := strings.Index(in, ".header"); idx != -1 {
		out = out[0:idx]
	}
	return out
}

// contains returns true iff b is completely inside a
func contains(a, b *Match) bool {
	return a.StartLine <= b.StartLine && a.EndLine >= b.EndLine
}

// returns true iff b <= a <= c
func between(a, b, c int) bool {
	return b <= a && a <= c
}

// returns true iff the ranges covered by a and b overlap.
func overlaps(a, b *Match) bool {
	return between(a.StartLine, b.StartLine, b.EndLine) || between(a.EndLine, b.StartLine, b.EndLine)
}

// endOfLicenseText is text commonly associated with the end of a license. We
// can remove text that occurs after it as well as the marker itself.
var endOfLicenseText = []string{
	"END OF TERMS AND CONDITIONS",
}

// trimExtraneousTrailingText removes text after an obvious end of the license
// and does not include substantive text of the license.
func trimExtraneousTrailingText(s string) string {
	for _, e := range endOfLicenseText {
		if i := strings.LastIndex(s, e); i != -1 {
			return s[:i]
		}
	}
	return s
}
