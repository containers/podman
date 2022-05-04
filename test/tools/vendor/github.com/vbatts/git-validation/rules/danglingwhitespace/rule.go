package danglingwhitespace

import (
	"github.com/vbatts/git-validation/git"
	"github.com/vbatts/git-validation/validate"
)

var (
	// DanglingWhitespace is the rule for checking the presence of dangling
	// whitespaces on line endings.
	DanglingWhitespace = validate.Rule{
		Name:        "dangling-whitespace",
		Description: "checking the presence of dangling whitespaces on line endings",
		Run:         ValidateDanglingWhitespace,
		Default:     true,
	}
)

func init() {
	validate.RegisterRule(DanglingWhitespace)
}

// ValidateDanglingWhitespace runs Git's check to look for whitespace errors.
func ValidateDanglingWhitespace(r validate.Rule, c git.CommitEntry) (vr validate.Result) {
	vr.CommitEntry = c
	vr.Msg = "commit does not have any whitespace errors"
	vr.Pass = true

	_, err := git.Check(c["commit"])
	if err != nil {
		vr.Pass = false
		if err.Error() == "exit status 2" {
			vr.Msg = "has whitespace errors. See `git show --check " + c["commit"] + "`."
		} else {
			vr.Msg = "errored with: " + err.Error()
		}
	}
	return
}
