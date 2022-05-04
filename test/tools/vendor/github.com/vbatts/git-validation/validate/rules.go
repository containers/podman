package validate

import (
	"sort"
	"strings"
	"sync"

	"github.com/vbatts/git-validation/git"
)

var (
	// RegisteredRules are the avaible validation to perform on git commits
	RegisteredRules  = []Rule{}
	registerRuleLock = sync.Mutex{}
)

// RegisterRule includes the Rule in the avaible set to use
func RegisterRule(vr Rule) {
	registerRuleLock.Lock()
	defer registerRuleLock.Unlock()
	RegisteredRules = append(RegisteredRules, vr)
}

// Rule will operate over a provided git.CommitEntry, and return a result.
type Rule struct {
	Name        string // short name for reference in in the `-run=...` flag
	Value       string // value to configure for the rule (i.e. a regexp to check for in the commit message)
	Description string // longer Description for readability
	Run         func(Rule, git.CommitEntry) Result
	Default     bool // whether the registered rule is run by default
}

// Commit processes the given rules on the provided commit, and returns the result set.
func Commit(c git.CommitEntry, rules []Rule) Results {
	results := Results{}
	for _, r := range rules {
		results = append(results, r.Run(r, c))
	}
	return results
}

// Result is the result for a single validation of a commit.
type Result struct {
	CommitEntry git.CommitEntry
	Pass        bool
	Msg         string
}

// Results is a set of results. This is type makes it easy for the following function.
type Results []Result

// PassFail gives a quick over/under of passes and failures of the results in this set
func (vr Results) PassFail() (pass int, fail int) {
	for _, res := range vr {
		if res.Pass {
			pass++
		} else {
			fail++
		}
	}
	return pass, fail
}

// SanitizeFilters takes a comma delimited list and returns the trimmend and
// split (on ",") items in the list
func SanitizeFilters(filtStr string) (filters []string) {
	for _, item := range strings.Split(filtStr, ",") {
		filters = append(filters, strings.TrimSpace(item))
	}
	return
}

// FilterRules takes a set of rules and a list of short names to include, and
// returns the reduced set.  The comparison is case insensitive.
//
// Some `includes` rules have values assigned to them.
// i.e. -run "dco,message_regexp='^JIRA-[0-9]+ [A-Z].*$'"
//
func FilterRules(rules []Rule, includes []string) []Rule {
	ret := []Rule{}

	for _, r := range rules {
		for i := range includes {
			if strings.Contains(includes[i], "=") {
				chunks := strings.SplitN(includes[i], "=", 2)
				if strings.ToLower(r.Name) == strings.ToLower(chunks[0]) {
					// for these rules, the Name won't be unique per se. There may be
					// multiple "regexp=" with different values. We'll need to set the
					// .Value = chunk[1] and ensure r is dup'ed so they don't clobber
					// each other.
					newR := Rule(r)
					newR.Value = chunks[1]
					ret = append(ret, newR)
				}
			} else {
				if strings.ToLower(r.Name) == strings.ToLower(includes[i]) {
					ret = append(ret, r)
				}
			}
		}
	}

	return ret
}

// StringsSliceEqual compares two string arrays for equality
func StringsSliceEqual(a, b []string) bool {
	if !sort.StringsAreSorted(a) {
		sort.Strings(a)
	}
	if !sort.StringsAreSorted(b) {
		sort.Strings(b)
	}
	for i := range b {
		if !StringsSliceContains(a, b[i]) {
			return false
		}
	}
	for i := range a {
		if !StringsSliceContains(b, a[i]) {
			return false
		}
	}
	return true
}

// StringsSliceContains checks for the presence of a word in string array
func StringsSliceContains(a []string, b string) bool {
	if !sort.StringsAreSorted(a) {
		sort.Strings(a)
	}
	i := sort.SearchStrings(a, b)
	return i < len(a) && a[i] == b
}
