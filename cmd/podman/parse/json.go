package parse

import "regexp"

var jsonFormatRegex = regexp.MustCompile(`^\s*(json|{{\s*json\s*(\.)?\s*}})\s*$`)

// MatchesJSONFormat test CLI --format string to be a JSON request
func MatchesJSONFormat(s string) bool {
	return jsonFormatRegex.Match([]byte(s))
}
