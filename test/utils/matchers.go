package utils

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/containers/common/pkg/config"
	. "github.com/onsi/gomega" //nolint:golint,stylecheck
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/matchers"
	"github.com/onsi/gomega/types"
)

// HaveActiveService verifies the given service is the active service
func HaveActiveService(name interface{}) OmegaMatcher {
	return WithTransform(
		func(cfg *config.Config) string {
			return cfg.Engine.ActiveService
		},
		Equal(name))
}

type ServiceMatcher struct {
	types.GomegaMatcher
	Name                  interface{}
	URI                   interface{}
	Identity              interface{}
	failureMessage        string
	negatedFailureMessage string
}

func VerifyService(name, uri, identity interface{}) OmegaMatcher {
	return &ServiceMatcher{
		Name:     name,
		URI:      uri,
		Identity: identity,
	}
}

func (matcher *ServiceMatcher) Match(actual interface{}) (success bool, err error) {
	cfg, ok := actual.(*config.Config)
	if !ok {
		return false, fmt.Errorf("ServiceMatcher matcher expects a config.Config")
	}

	if _, err = url.Parse(matcher.URI.(string)); err != nil {
		return false, err
	}

	success, err = HaveKey(matcher.Name).Match(cfg.Engine.ServiceDestinations)
	if !success || err != nil {
		matcher.failureMessage = HaveKey(matcher.Name).FailureMessage(cfg.Engine.ServiceDestinations)
		matcher.negatedFailureMessage = HaveKey(matcher.Name).NegatedFailureMessage(cfg.Engine.ServiceDestinations)
		return
	}

	sd := cfg.Engine.ServiceDestinations[matcher.Name.(string)]
	success, err = Equal(matcher.URI).Match(sd.URI)
	if !success || err != nil {
		matcher.failureMessage = Equal(matcher.URI).FailureMessage(sd.URI)
		matcher.negatedFailureMessage = Equal(matcher.URI).NegatedFailureMessage(sd.URI)
		return
	}

	success, err = Equal(matcher.Identity).Match(sd.Identity)
	if !success || err != nil {
		matcher.failureMessage = Equal(matcher.Identity).FailureMessage(sd.Identity)
		matcher.negatedFailureMessage = Equal(matcher.Identity).NegatedFailureMessage(sd.Identity)
		return
	}

	return true, nil
}

func (matcher *ServiceMatcher) FailureMessage(_ interface{}) string {
	return matcher.failureMessage
}

func (matcher *ServiceMatcher) NegatedFailureMessage(_ interface{}) string {
	return matcher.negatedFailureMessage
}

type URLMatcher struct {
	matchers.EqualMatcher
}

// VerifyURL matches when actual is a valid URL and matches expected
func VerifyURL(uri interface{}) OmegaMatcher {
	return &URLMatcher{matchers.EqualMatcher{Expected: uri}}
}

func (matcher *URLMatcher) Match(actual interface{}) (bool, error) {
	e, ok := matcher.Expected.(string)
	if !ok {
		return false, fmt.Errorf("VerifyURL requires string inputs %T is not supported", matcher.Expected)
	}
	eURI, err := url.Parse(e)
	if err != nil {
		return false, err
	}

	a, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("VerifyURL requires string inputs %T is not supported", actual)
	}
	aURI, err := url.Parse(a)
	if err != nil {
		return false, err
	}

	return (&matchers.EqualMatcher{Expected: eURI}).Match(aURI)
}

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

// Match follows gexec.Matcher interface
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
		return false, nil
	}
	return true, nil
}

func (matcher *ValidJSONMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be valid JSON")
}

func (matcher *ValidJSONMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to _not_ be valid JSON")
}
