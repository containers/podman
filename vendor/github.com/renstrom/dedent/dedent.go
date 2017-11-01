package dedent

import (
	"regexp"
	"strings"
)

var whitespaceOnly = regexp.MustCompile("(?m)^[ \t]+$")
var leadingWhitespace = regexp.MustCompile("(?m)(^[ \t]*)")

// Dedent removes any common leading whitespace from every line in s.
//
// This can be used to make multiline strings to line up with the left edge of
// the display, while still presenting them in the source code in indented
// form.
func Dedent(s string) string {
	s = whitespaceOnly.ReplaceAllString(s, "")
	margin := findMargin(s)
	if len(margin) == 0 {
		return s
	}
	return regexp.MustCompile("(?m)^"+margin).ReplaceAllString(s, "")
}

// Look for the longest leading string of spaces and tabs common to all lines.
func findMargin(s string) string {
	var margin string

	indents := leadingWhitespace.FindAllString(s, -1)
	numIndents := len(indents)
	for i, indent := range indents {
		// Don't use last row if it is empty
		if i == numIndents-1 && indent == "" {
			break
		}

		if margin == "" {
			margin = indent
		} else if strings.HasPrefix(indent, margin) {
			// Current line more deeply indented than previous winner:
			// no change (previous winner is still on top).
			continue
		} else if strings.HasPrefix(margin, indent) {
			// Current line consistent with and no deeper than previous winner:
			// it's the new winner.
			margin = indent
		} else {
			// Current line and previous winner have no common whitespace:
			// there is no margin.
			margin = ""
			break
		}
	}

	return margin
}
