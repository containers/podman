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

type frequencyTable struct {
	counts map[tokenID]int // key: token ID, value: number of instances of that token
}

func newFrequencyTable() *frequencyTable {
	return &frequencyTable{
		counts: make(map[tokenID]int),
	}
}

func (f *frequencyTable) update(d *indexedDocument) {
	for _, tok := range d.Tokens {
		f.counts[tok.ID]++
	}
}

func (d *indexedDocument) generateFrequencies() {
	d.f = newFrequencyTable()
	d.f.update(d)
}

// TokenSimilarity returns a confidence score of how well d contains
// the tokens of o. This is used as a fast similarity metric to
// avoid running more expensive classifiers.
func (d *indexedDocument) tokenSimilarity(o *indexedDocument) float64 {
	hits := 0
	// For each token in the source document, see if the target has "enough" instances
	// of that token to possibly be a match to the target.
	// We count up all the matches, and divide by the total number of unique source
	// tokens to get a similarity metric. 1.0 means that all the tokens in the target
	// are present in the source in appropriate quantities. If the value here is lower
	// than the desired matching threshold, the target can't possibly match the source.
	// Profiling indicates a significant amount of time is spent here.
	// Avoiding checking (or storing) "uninteresting" tokens (common English words)
	// could help.
	for t, c := range o.f.counts {
		if d.f.counts[t] >= c {
			hits++
		}
	}

	return float64(hits) / float64(len(o.f.counts))
}
