package report

import "regexp"

var jsonRegex = regexp.MustCompile(`^\s*(json|{{\s*json\s*(\.)?\s*}})\s*$`)

// JSONFormat test CLI --format string to be a JSON request
// if report.IsJSON(cmd.Flag("format").Value.String()) {
//   ... process JSON and output
// }
func IsJSON(s string) bool {
	return jsonRegex.MatchString(s)
}
