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
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// isSignificant looks for runes that are likely to be the part of English language content
// of interest in licenses. Notably, it skips over punctuation, looking only for letters
// or numbers that consistitute the tokens of most interest.
func isSignificant(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

var eol = "\n"

func cleanupToken(in string) string {
	r, _ := utf8.DecodeRuneInString(in)
	var out strings.Builder
	if !unicode.IsLetter(r) {
		if unicode.IsDigit(r) {
			// Based on analysis of the license corpus, the characters
			// that are significant are numbers, periods, and dashes. Anything
			// else can be safely discarded, and helps avoid matching failures
			// due to inconsistent whitespacing and formatting.
			for _, c := range in {
				if unicode.IsDigit(c) || c == '.' || c == '-' {
					out.WriteRune(c)
				}
			}

			// Numbers should not end in a . since that doesn't indicate a version
			// number, but usually an end of a line.
			res := out.String()
			for strings.HasSuffix(res, ".") {
				res = res[0 : len(res)-1]
			}
			return res
		}
	}

	// Remove internal hyphenization or URL constructs to better normalize
	// strings for matching.
	for _, c := range in {
		if c >= 'a' && c <= 'z' {
			out.WriteRune(c)
		}
	}
	return out.String()
}

// tokenize produces a document from the input content.
func tokenize(in []byte) *document {
	// Apply the global transforms described in SPDX

	norm := strings.ToLower(string(in))
	norm = html.UnescapeString(norm)
	norm = normalizePunctuation(norm)
	norm = normalizeEquivalentWords(norm)
	norm = removeIgnorableTexts(norm)

	var doc document
	// Iterate on a line-by-line basis.

	line := norm
	i := 0
	pos := 0
	for {
		// Scan the line for the first likely textual content. The scan ignores punctuation
		// artifacts that include visual boxes for layout as well as comment characters in
		// source files.
		firstInLine := true
		var wid int
		var r rune

		if pos == len(line) {
			break
		}

		next := func() {
			r, wid = utf8.DecodeRuneInString(line[pos:])
			pos += wid
		}

		for pos < len(line) {
			start := pos
			next()

			if r == '\n' {
				doc.Tokens = append(doc.Tokens, &token{
					Text: eol,
					Line: i + 1})
				i++
			}

			if !isSignificant(r) {
				continue
			}

			// We're at a word/number character.
			for pos < len(line) {
				next()
				if unicode.IsSpace(r) {
					pos -= wid // Will skip this in outer loop
					break
				}
			}

			if pos > start {
				if start >= 2 && line[start-2] == '.' && line[start-1] == ' ' {
					// Insert a "soft EOL" that helps detect header-looking entries that
					// follow this text. This resolves problems with licenses that are a
					// very long line of text, motivated by
					// https://github.com/microsoft/TypeScript/commit/6e6e570d57b6785335668e30b63712e41f89bf74#diff-e60c8cd1bc09b7c4e1bf79c769c9c120L109
					doc.Tokens = append(doc.Tokens, &token{
						Text: eol,
						Line: i + 1})
				}

				tok := token{
					Text: line[start:pos],
					Line: i + 1,
				}
				if firstInLine {
					// Store the prefix material, it is useful to discern some corner cases
					tok.Previous = line[0:start]
				}
				doc.Tokens = append(doc.Tokens, &tok)
				firstInLine = false
			}
		}
		tok := token{
			Text: eol,
			Line: i + 1,
		}
		doc.Tokens = append(doc.Tokens, &tok)
	}
	doc.Tokens = cleanupTokens(doc.Tokens)
	return &doc
}

func cleanupTokens(in []*token) []*token {
	// This routine performs sanitization of tokens. If it is a header-looking
	// token (but not a version number) starting a line, it is removed.
	// Hyphenated words are reassembled.
	partialWord := ""
	var out []*token
	tokIdx := 0
	firstInLine := true
	for i, tok := range in {
		if firstInLine && header(tok) {
			continue
		}
		if tok.Text == eol {
			firstInLine = true
			continue
		}
		firstInLine = false
		t := cleanupToken(tok.Text)
		// If this is the last token in a line, and it looks like a hyphenated
		// word, store it for reassembly.
		if strings.HasSuffix(tok.Text, "-") && in[i+1].Text == eol {
			partialWord = t
		} else if partialWord != "" {
			// Repair hyphenated words
			tp := in[i-1]
			tp.Text = partialWord + t
			tp.Index = tokIdx
			tp.Previous = ""
			out = append(out, tp)
			tokIdx++
			partialWord = ""
		} else {
			tok.Text = t
			tok.Index = tokIdx
			tok.Previous = ""
			out = append(out, tok)
			tokIdx++
		}
	}
	return out
}

// interchangeablePunctutation is punctuation that can be normalized.
var interchangeablePunctuation = []struct {
	interchangeable string
	substitute      string
}{
	// Hyphen, Dash, En Dash, and Em Dash.
	{`-‒–—‐`, "-"},
	// Single, Double, Curly Single, and Curly Double.
	{"'\"`‘’“”", "'"},
	// Copyright.
	{"©", "(c)"},
	// Currency and Section. (Different copies of the CDDL use each marker.)
	{"§¤", "(s)"},
	// Middle Dot
	{"·", " "},
	{"*", " "},
}

// normalizePunctuation takes all hyphens and quotes and normalizes them.
func normalizePunctuation(s string) string {
	for _, iw := range interchangeablePunctuation {
		for _, in := range strings.Split(iw.interchangeable, "") {
			s = strings.ReplaceAll(s, in, iw.substitute)
		}
	}
	return s
}

// interchangeableWords are words we can substitute for a normalized form
// without changing the meaning of the license. See
// https://spdx.org/spdx-license-list/matching-guidelines for the list.
var interchangeableWords = []struct {
	interchangeable *regexp.Regexp
	substitute      string
}{
	{regexp.MustCompile("acknowledgement"), "acknowledgment"},
	{regexp.MustCompile("analogue"), "analog"},
	{regexp.MustCompile("analyse"), "analyze"},
	{regexp.MustCompile("artefact"), "artifact"},
	{regexp.MustCompile("authorisation"), "authorization"},
	{regexp.MustCompile("authorised"), "authorized"},
	{regexp.MustCompile("calibre"), "caliber"},
	{regexp.MustCompile("cancelled"), "canceled"},
	{regexp.MustCompile("capitalisations"), "capitalizations"},
	{regexp.MustCompile("catalogue"), "catalog"},
	{regexp.MustCompile("categorise"), "categorize"},
	{regexp.MustCompile("centre"), "center"},
	{regexp.MustCompile("emphasised"), "emphasized"},
	{regexp.MustCompile("favour"), "favor"},
	{regexp.MustCompile("favourite"), "favorite"},
	{regexp.MustCompile("fulfil\\b"), "fulfill"},
	{regexp.MustCompile("fulfilment"), "fulfillment"},
	{regexp.MustCompile("https"), "http"},
	{regexp.MustCompile("initialise"), "initialize"},
	{regexp.MustCompile("judgment"), "judgement"},
	{regexp.MustCompile("labelling"), "labeling"},
	{regexp.MustCompile("labour"), "labor"},
	{regexp.MustCompile("licence"), "license"},
	{regexp.MustCompile("maximise"), "maximize"},
	{regexp.MustCompile("modelled"), "modeled"},
	{regexp.MustCompile("modelling"), "modeling"},
	{regexp.MustCompile("offence"), "offense"},
	{regexp.MustCompile("optimise"), "optimize"},
	{regexp.MustCompile("organisation"), "organization"},
	{regexp.MustCompile("organise"), "organize"},
	{regexp.MustCompile("practise"), "practice"},
	{regexp.MustCompile("programme"), "program"},
	{regexp.MustCompile("realise"), "realize"},
	{regexp.MustCompile("recognise"), "recognize"},
	{regexp.MustCompile("signalling"), "signaling"},
	{regexp.MustCompile("sub[ -]license"), "sublicense"},
	{regexp.MustCompile("utilisation"), "utilization"},
	{regexp.MustCompile("whilst"), "while"},
	{regexp.MustCompile("wilful"), "wilfull"},
	{regexp.MustCompile("non[ -]commercial"), "noncommercial"},
	{regexp.MustCompile("per cent"), "percent"},
}

// normalizeEquivalentWords normalizes equivalent words that are interchangeable.
func normalizeEquivalentWords(s string) string {
	for _, iw := range interchangeableWords {
		s = iw.interchangeable.ReplaceAllString(s, iw.substitute)
	}
	return s
}

func header(tok *token) bool {
	in := tok.Text
	p, e := in[:len(in)-1], in[len(in)-1]
	switch e {
	case '.', ':', ')':
		if listMarker[p] {
			if e != ')' {
				return true
			}
			// Sometimes an internal reference like "(ii)" from NPL-1.02.txt
			// endds up at the beginning of a line. In that case, it's
			// not actually a header.
			if e == ')' && !strings.HasSuffix(tok.Previous, "(") {
				return true
			}
		}
		// Check for patterns like 1.2.3
		for _, r := range p {
			if unicode.IsDigit(r) || r == '.' {
				continue
			}
			return false
		}
		return true
	}
	return false
}

var listMarker = func() map[string]bool {
	const allListMarkers = "a b c d e f g h i j k l m n o p q r ii iii iv v vi vii viii ix xi xii xiii xiv xv"
	l := map[string]bool{}
	for _, marker := range strings.Split(allListMarkers, " ") {
		l[marker] = true
	}
	return l
}()

// ignorableTexts is a list of lines at the start of the string we can remove
// to get a cleaner match.
var ignorableTexts = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(?:the )?mit license(?: \(mit\))?$`),
	regexp.MustCompile(`(?i)^(?:new )?bsd license$`),
	regexp.MustCompile(`(?i)^copyright and permission notice$`),
	regexp.MustCompile(`^(.{1,5})?copyright (\(c\) )?(\[yyyy\]|\d{4})[,.]?.*$`),
	regexp.MustCompile(`^(.{1,5})?copyright \(c\) \[dates of first publication\].*$`),
	regexp.MustCompile(`^\d{4}-(\d{2}|[a-z]{3})-\d{2}$`),
	regexp.MustCompile(`^\d{4}-[a-z]{3}-\d{2}$`),
	regexp.MustCompile(`(?i)^(all|some) rights reserved\.?$`),
	regexp.MustCompile(`(?i)^@license$`),
	regexp.MustCompile(`^\s*$`),
}

// removeIgnorableTexts removes common text, which is not important for
// classification
func removeIgnorableTexts(s string) string {
	var out []string
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for _, l := range lines {
		line := strings.TrimSpace(l)
		var match bool
		for _, re := range ignorableTexts {
			if re.MatchString(line) {
				match = true
			}
		}
		if !match {
			out = append(out, l)
		} else {
			// We want to preserve line presence for the positional information
			out = append(out, "")
		}
	}
	return strings.Join(out, "\n") + "\n"
}
