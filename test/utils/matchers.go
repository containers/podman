package utils

import (
	"fmt"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/gexec"
)

// ExitWithError matches when assertion is > argument.  Default 0
// Modeled after the gomega Exit() matcher
func ExitWithError(optionalExitCode ...int) *exitMatcher {
	exitCode := 0
	if len(optionalExitCode) > 0 {
		exitCode = optionalExitCode[0]
	}
	return &exitMatcher{exitCode: exitCode}
}

type exitMatcher struct {
	exitCode       int
	actualExitCode int
}

func (m *exitMatcher) Match(actual interface{}) (success bool, err error) {
	exiter, ok := actual.(gexec.Exiter)
	if !ok {
		return false, fmt.Errorf("ExitWithError must be passed a gexec.Exiter (Missing method ExitCode() int) Got:\n#{format.Object(actual, 1)}")
	}

	m.actualExitCode = exiter.ExitCode()
	if m.actualExitCode == -1 {
		return false, nil
	}
	return m.actualExitCode > m.exitCode, nil
}

func (m *exitMatcher) FailureMessage(actual interface{}) (message string) {
	if m.actualExitCode == -1 {
		return "Expected process to exit.  It did not."
	}
	return format.Message(m.actualExitCode, "to be greater than exit code:", m.exitCode)
}

func (m *exitMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	if m.actualExitCode == -1 {
		return "you really shouldn't be able to see this!"
	} else {
		if m.exitCode == -1 {
			return "Expected process not to exit.  It did."
		}
		return format.Message(m.actualExitCode, "is less than or equal to exit code:", m.exitCode)
	}
}
func (m *exitMatcher) MatchMayChangeInTheFuture(actual interface{}) bool {
	session, ok := actual.(*gexec.Session)
	if ok {
		return session.ExitCode() == -1
	}
	return true
}
