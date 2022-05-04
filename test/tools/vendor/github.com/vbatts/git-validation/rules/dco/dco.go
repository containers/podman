package dco

import (
	"regexp"
	"strings"

	"github.com/vbatts/git-validation/git"
	"github.com/vbatts/git-validation/validate"
)

func init() {
	validate.RegisterRule(DcoRule)
}

var (
	// ValidDCO is the regexp for signed off DCO
	ValidDCO = regexp.MustCompile(`^Signed-off-by: ([^<]+) <([^<>@]+@[^<>]+)>$`)
	// DcoRule is the rule being registered
	DcoRule = validate.Rule{
		Name:        "DCO",
		Description: "makes sure the commits are signed",
		Run:         ValidateDCO,
		Default:     true,
	}
)

// ValidateDCO checks that the commit has been signed off, per the DCO process
func ValidateDCO(r validate.Rule, c git.CommitEntry) (vr validate.Result) {
	vr.CommitEntry = c
	if len(strings.Split(c["parent"], " ")) > 1 {
		vr.Pass = true
		vr.Msg = "merge commits do not require DCO"
		return vr
	}

	hasValid := false
	for _, line := range strings.Split(c["body"], "\n") {
		if ValidDCO.MatchString(line) {
			hasValid = true
		}
	}
	if !hasValid {
		vr.Pass = false
		vr.Msg = "does not have a valid DCO"
	} else {
		vr.Pass = true
		vr.Msg = "has a valid DCO"
	}

	return vr
}
