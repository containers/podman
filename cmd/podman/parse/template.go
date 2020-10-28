package parse

import (
	"regexp"
	"strings"
)

var rangeRegex = regexp.MustCompile(`{{\s*range\s*\.\s*}}.*{{\s*end\s*}}`)

// TODO move to github.com/containers/common/pkg/report
// EnforceRange ensures that the format string contains a range
func EnforceRange(format string) string {
	if !rangeRegex.MatchString(format) {
		return "{{range .}}" + format + "{{end}}"
	}
	return format
}

// EnforceRange ensures that the format string contains a range
func HasTable(format string) bool {
	return strings.HasPrefix(format, "table ")
}
