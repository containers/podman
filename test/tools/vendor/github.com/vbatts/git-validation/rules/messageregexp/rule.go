package messageregexp

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/vbatts/git-validation/git"
	"github.com/vbatts/git-validation/validate"
)

func init() {
	validate.RegisterRule(RegexpRule)
}

var (
	// RegexpRule for validating a user provided regex on the commit messages
	RegexpRule = validate.Rule{
		Name:        "message_regexp",
		Description: "checks the commit message for a user provided regular expression",
		Run:         ValidateMessageRegexp,
		Default:     false, // only for users specifically calling it through -run ...
	}
)

// ValidateMessageRegexp is the message regex func to run
func ValidateMessageRegexp(r validate.Rule, c git.CommitEntry) (vr validate.Result) {
	if r.Value == "" {
		vr.Pass = true
		vr.Msg = "noop: message_regexp value is blank"
		return vr
	}

	re := regexp.MustCompile(r.Value)
	vr.CommitEntry = c
	if len(strings.Split(c["parent"], " ")) > 1 {
		vr.Pass = true
		vr.Msg = "merge commits are not checked for message_regexp"
		return vr
	}

	hasValid := false
	for _, line := range strings.Split(c["subject"], "\n") {
		if re.MatchString(line) {
			hasValid = true
		}
	}
	for _, line := range strings.Split(c["body"], "\n") {
		if re.MatchString(line) {
			hasValid = true
		}
	}
	if !hasValid {
		vr.Pass = false
		vr.Msg = fmt.Sprintf("commit message does not match %q", r.Value)
	} else {
		vr.Pass = true
		vr.Msg = fmt.Sprintf("commit message matches %q", r.Value)
	}
	return vr
}
