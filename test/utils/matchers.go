package utils

import (
	"encoding/json"
	"fmt"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
)

type ExitMatcher struct {
	types.GomegaMatcher
	Expected int
	Actual   int
}

// ExitWithError matches when assertion is > argument.  Default 0
// Modeled after the gomega Exit() matcher and also operates on sessions.
func ExitWithError(optionalExitCode ...int) *ExitMatcher {
	exitCode := 0
	if len(optionalExitCode) > 0 {
		exitCode = optionalExitCode[0]
	}
	return &ExitMatcher{Expected: exitCode}
}

// Match follows gexec.Matcher interface.
func (matcher *ExitMatcher) Match(actual interface{}) (success bool, err error) {
	exiter, ok := actual.(gexec.Exiter)
	if !ok {
		return false, fmt.Errorf("ExitWithError must be passed a gexec.Exiter (Missing method ExitCode() int) Got:\n#{format.Object(actual, 1)}")
	}

	matcher.Actual = exiter.ExitCode()
	if matcher.Actual == -1 {
		return false, nil
	}
	return matcher.Actual > matcher.Expected, nil
}

func (matcher *ExitMatcher) FailureMessage(_ interface{}) (message string) {
	if matcher.Actual == -1 {
		return "Expected process to exit.  It did not."
	}
	return format.Message(matcher.Actual, "to be greater than exit code: ", matcher.Expected)
}

func (matcher *ExitMatcher) NegatedFailureMessage(_ interface{}) (message string) {
	switch {
	case matcher.Actual == -1:
		return "you really shouldn't be able to see this!"
	case matcher.Expected == -1:
		return "Expected process not to exit.  It did."
	}
	return format.Message(matcher.Actual, "is less than or equal to exit code: ", matcher.Expected)
}

func (matcher *ExitMatcher) MatchMayChangeInTheFuture(actual interface{}) bool {
	session, ok := actual.(*gexec.Session)
	if ok {
		return session.ExitCode() == -1
	}
	return true
}

// ExitCleanly asserts that a PodmanSession exits 0 and with no stderr
func ExitCleanly() types.GomegaMatcher {
	return &exitCleanlyMatcher{}
}

type exitCleanlyMatcher struct {
	msg string
}

type podmanSession interface {
	ExitCode() int
	ErrorToString() string
}

func (matcher *exitCleanlyMatcher) Match(actual interface{}) (success bool, err error) {
	session, ok := actual.(podmanSession)
	if !ok {
		return false, fmt.Errorf("ExitCleanly must be passed a PodmanSession; Got:\n %+v\n%q", actual, format.Object(actual, 1))
	}

	exitcode := session.ExitCode()
	stderr := session.ErrorToString()
	if exitcode != 0 {
		matcher.msg = fmt.Sprintf("Command failed with exit status %d", exitcode)
		if stderr != "" {
			matcher.msg += ". See above for error message."
		}
		return false, nil
	}

	// Exit status is 0. Now check for anything on stderr
	if stderr != "" {
		matcher.msg = fmt.Sprintf("Unexpected warnings seen on stderr: %q", stderr)
		return false, nil
	}

	return true, nil
}

func (matcher *exitCleanlyMatcher) FailureMessage(_ interface{}) (message string) {
	return matcher.msg
}

func (matcher *exitCleanlyMatcher) NegatedFailureMessage(_ interface{}) (message string) {
	// FIXME - I see no situation in which we could ever want this?
	return matcher.msg + " (NOT!)"
}

type ValidJSONMatcher struct {
	types.GomegaMatcher
}

func BeValidJSON() *ValidJSONMatcher {
	return &ValidJSONMatcher{}
}

func (matcher *ValidJSONMatcher) Match(actual interface{}) (success bool, err error) {
	s, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("ValidJSONMatcher expects a string, not %q", actual)
	}

	var i interface{}
	if err := json.Unmarshal([]byte(s), &i); err != nil {
		return false, err
	}
	return true, nil
}

func (matcher *ValidJSONMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be valid JSON")
}

func (matcher *ValidJSONMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to _not_ be valid JSON")
}
