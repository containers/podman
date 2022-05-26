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
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// This file contains word-diffing routines that build on top of the go-diff package.
// The algorithm implemented here is from the suggested word diffing technique in
// https://github.com/google/diff-match-patch/wiki/Line-or-Word-Diffs

// diffRange returns the indices of the beginning and end locations of the diff
// that reconstruct (as best possible) the source value.
func diffRange(known string, diffs []diffmatchpatch.Diff) (start, end int) {
	var foundStart bool
	var seen string
	for end = 0; end < len(diffs); end++ {
		if len(seen) > 1 && seen[:len(seen)-1] == known {
			break
		}
		switch diffs[end].Type {
		case diffmatchpatch.DiffEqual, diffmatchpatch.DiffInsert:
			if !foundStart {
				start = end
				foundStart = true
			}
			seen += diffs[end].Text + " "
		}
	}
	return start, end
}

func docDiff(id string, doc1 *indexedDocument, doc1Start, doc1End int, doc2 *indexedDocument, doc2Start, doc2End int) []diffmatchpatch.Diff {
	chars1 := doc1.runes[doc1Start:doc1End]
	chars2 := doc2.runes[doc2Start:doc2End]

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMainRunes(chars1, chars2, false)

	// Recover the words from the previous rune encoding and return the textual diffs.
	diffs = diffRunesToWords(diffs, doc1.dict)
	return diffs
}

func diffWordsToRunes(doc *indexedDocument, start, end int) []rune {
	// Creates a slice of runes using the indexed values as a basis for runes.
	// The go-diff code basically does exactly this using ephemeral dictionaries
	// for each input string. We leverage the fact we have a persistent dictionary
	// to make this operation cheaper.
	// TODO: perhaps we should cache these in the corpus?
	runes := make([]rune, 0, end-start)

	for _, t := range doc.Tokens[start:end] {
		runes = append(runes, rune(t.ID))
	}
	return runes
}

// diffRunesToWords rehydrates the text in a diff from a string of word hashes to real words of text.
func diffRunesToWords(diffs []diffmatchpatch.Diff, dict *dictionary) []diffmatchpatch.Diff {
	hydrated := make([]diffmatchpatch.Diff, 0, len(diffs))
	for _, aDiff := range diffs {
		chars := []rune(aDiff.Text)
		var sb strings.Builder

		for i, r := range chars {
			sb.WriteString(dict.getWord(tokenID(r)))
			if (i + 1) < len(chars) {
				sb.WriteByte(' ')
			}
		}

		aDiff.Text = sb.String()
		hydrated = append(hydrated, aDiff)
	}
	return hydrated
}

// Returns the number of words in the input string. Used by scoring and distance functions.
// This function depends on the behavior of the tokenizer such that strings are separated
// by exactly one space and don't start or end with whitespace.
func wordLen(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, " ") + 1
}

// textLength returns the number of tokens in the diff. This value is used to
// adjust the offset for detection, since this is the number of tokens
// discarded while matching a diff.  By virtue of how it's called, there won't
// be "change" diffs (a paired insert/delete) so we can simplify the scan to
// just count up everything.
func textLength(diffs []diffmatchpatch.Diff) int {
	l := 0
	for _, d := range diffs {
		l += wordLen(d.Text)
	}
	return l
}
