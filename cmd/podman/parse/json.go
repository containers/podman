package parse

import "regexp"

var jsonFormatRegex = regexp.MustCompile(`^(\s*json\s*|\s*{{\s*json\s*\.\s*}}\s*)$`)

func MatchesJSONFormat(s string) bool {
	return jsonFormatRegex.Match([]byte(s))
}
