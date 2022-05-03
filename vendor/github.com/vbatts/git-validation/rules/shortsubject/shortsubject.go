package shortsubject

import (
	"strings"

	"github.com/vbatts/git-validation/git"
	"github.com/vbatts/git-validation/validate"
)

var (
	// ShortSubjectRule is the rule being registered
	ShortSubjectRule = validate.Rule{
		Name:        "short-subject",
		Description: "commit subjects are strictly less than 90 (github ellipsis length)",
		Run:         ValidateShortSubject,
		Default:     true,
	}
)

func init() {
	validate.RegisterRule(ShortSubjectRule)
}

// ValidateShortSubject checks that the commit's subject is strictly less than
// 90 characters (preferably not more than 72 chars).
func ValidateShortSubject(r validate.Rule, c git.CommitEntry) (vr validate.Result) {
	if len(strings.Split(c["parent"], " ")) > 1 {
		vr.Pass = true
		vr.Msg = "merge commits do not require length check"
		return vr
	}
	if len(c["subject"]) >= 90 {
		vr.Pass = false
		vr.Msg = "commit subject exceeds 90 characters"
		return
	}
	vr.Pass = true
	if len(c["subject"]) > 72 {
		vr.Msg = "commit subject is under 90 characters, but is still more than 72 chars"
	} else {
		vr.Msg = "commit subject is 72 characters or less! *yay*"
	}
	return
}
