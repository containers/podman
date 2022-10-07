package internal

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/onsi/gomega/types"
)

type AsyncAssertionType uint

const (
	AsyncAssertionTypeEventually AsyncAssertionType = iota
	AsyncAssertionTypeConsistently
)

type AsyncAssertion struct {
	asyncType AsyncAssertionType

	actualIsFunc bool
	actualValue  interface{}
	actualFunc   func() ([]reflect.Value, error)

	timeoutInterval time.Duration
	pollingInterval time.Duration
	ctx             context.Context
	offset          int
	g               *Gomega
}

func NewAsyncAssertion(asyncType AsyncAssertionType, actualInput interface{}, g *Gomega, timeoutInterval time.Duration, pollingInterval time.Duration, ctx context.Context, offset int) *AsyncAssertion {
	out := &AsyncAssertion{
		asyncType:       asyncType,
		timeoutInterval: timeoutInterval,
		pollingInterval: pollingInterval,
		offset:          offset,
		ctx:             ctx,
		g:               g,
	}

	switch actualType := reflect.TypeOf(actualInput); {
	case actualInput == nil || actualType.Kind() != reflect.Func:
		out.actualValue = actualInput
	case actualType.NumIn() == 0 && actualType.NumOut() > 0:
		out.actualIsFunc = true
		out.actualFunc = func() ([]reflect.Value, error) {
			return reflect.ValueOf(actualInput).Call([]reflect.Value{}), nil
		}
	case actualType.NumIn() == 1 && actualType.In(0).Implements(reflect.TypeOf((*types.Gomega)(nil)).Elem()):
		out.actualIsFunc = true
		out.actualFunc = func() (values []reflect.Value, err error) {
			var assertionFailure error
			assertionCapturingGomega := NewGomega(g.DurationBundle).ConfigureWithFailHandler(func(message string, callerSkip ...int) {
				skip := 0
				if len(callerSkip) > 0 {
					skip = callerSkip[0]
				}
				_, file, line, _ := runtime.Caller(skip + 1)
				assertionFailure = fmt.Errorf("Assertion in callback at %s:%d failed:\n%s", file, line, message)
				panic("stop execution")
			})

			defer func() {
				if actualType.NumOut() == 0 {
					if assertionFailure == nil {
						values = []reflect.Value{reflect.Zero(reflect.TypeOf((*error)(nil)).Elem())}
					} else {
						values = []reflect.Value{reflect.ValueOf(assertionFailure)}
					}
				} else {
					err = assertionFailure
				}
				if e := recover(); e != nil && assertionFailure == nil {
					panic(e)
				}
			}()

			values = reflect.ValueOf(actualInput).Call([]reflect.Value{reflect.ValueOf(assertionCapturingGomega)})
			return
		}
	default:
		msg := fmt.Sprintf("The function passed to Gomega's async assertions should either take no arguments and return values, or take a single Gomega interface that it can use to make assertions within the body of the function.  When taking a Gomega interface the function can optionally return values or return nothing.  The function you passed takes %d arguments and returns %d values.", actualType.NumIn(), actualType.NumOut())
		g.Fail(msg, offset+4)
	}

	return out
}

func (assertion *AsyncAssertion) WithOffset(offset int) types.AsyncAssertion {
	assertion.offset = offset
	return assertion
}

func (assertion *AsyncAssertion) WithTimeout(interval time.Duration) types.AsyncAssertion {
	assertion.timeoutInterval = interval
	return assertion
}

func (assertion *AsyncAssertion) WithPolling(interval time.Duration) types.AsyncAssertion {
	assertion.pollingInterval = interval
	return assertion
}

func (assertion *AsyncAssertion) Within(timeout time.Duration) types.AsyncAssertion {
	assertion.timeoutInterval = timeout
	return assertion
}

func (assertion *AsyncAssertion) ProbeEvery(interval time.Duration) types.AsyncAssertion {
	assertion.pollingInterval = interval
	return assertion
}

func (assertion *AsyncAssertion) WithContext(ctx context.Context) types.AsyncAssertion {
	assertion.ctx = ctx
	return assertion
}

func (assertion *AsyncAssertion) Should(matcher types.GomegaMatcher, optionalDescription ...interface{}) bool {
	assertion.g.THelper()
	vetOptionalDescription("Asynchronous assertion", optionalDescription...)
	return assertion.match(matcher, true, optionalDescription...)
}

func (assertion *AsyncAssertion) ShouldNot(matcher types.GomegaMatcher, optionalDescription ...interface{}) bool {
	assertion.g.THelper()
	vetOptionalDescription("Asynchronous assertion", optionalDescription...)
	return assertion.match(matcher, false, optionalDescription...)
}

func (assertion *AsyncAssertion) buildDescription(optionalDescription ...interface{}) string {
	switch len(optionalDescription) {
	case 0:
		return ""
	case 1:
		if describe, ok := optionalDescription[0].(func() string); ok {
			return describe() + "\n"
		}
	}
	return fmt.Sprintf(optionalDescription[0].(string), optionalDescription[1:]...) + "\n"
}

func (assertion *AsyncAssertion) pollActual() (interface{}, error) {
	if !assertion.actualIsFunc {
		return assertion.actualValue, nil
	}

	values, err := assertion.actualFunc()
	if err != nil {
		return nil, err
	}
	extras := []interface{}{nil}
	for _, value := range values[1:] {
		extras = append(extras, value.Interface())
	}
	success, message := vetActuals(extras, 0)
	if !success {
		return nil, errors.New(message)
	}

	return values[0].Interface(), nil
}

func (assertion *AsyncAssertion) matcherMayChange(matcher types.GomegaMatcher, value interface{}) bool {
	if assertion.actualIsFunc {
		return true
	}
	return types.MatchMayChangeInTheFuture(matcher, value)
}

type contextWithAttachProgressReporter interface {
	AttachProgressReporter(func() string) func()
}

func (assertion *AsyncAssertion) match(matcher types.GomegaMatcher, desiredMatch bool, optionalDescription ...interface{}) bool {
	timer := time.Now()
	timeout := time.After(assertion.timeoutInterval)
	lock := sync.Mutex{}

	var matches bool
	var err error
	mayChange := true

	value, err := assertion.pollActual()
	if err == nil {
		mayChange = assertion.matcherMayChange(matcher, value)
		matches, err = matcher.Match(value)
	}

	assertion.g.THelper()

	messageGenerator := func() string {
		// can be called out of band by Ginkgo if the user requests a progress report
		lock.Lock()
		defer lock.Unlock()
		errMsg := ""
		message := ""
		if err != nil {
			errMsg = "Error: " + err.Error()
		} else {
			if desiredMatch {
				message = matcher.FailureMessage(value)
			} else {
				message = matcher.NegatedFailureMessage(value)
			}
		}
		description := assertion.buildDescription(optionalDescription...)
		return fmt.Sprintf("%s%s%s", description, message, errMsg)
	}

	fail := func(preamble string) {
		assertion.g.THelper()
		assertion.g.Fail(fmt.Sprintf("%s after %.3fs.\n%s", preamble, time.Since(timer).Seconds(), messageGenerator()), 3+assertion.offset)
	}

	var contextDone <-chan struct{}
	if assertion.ctx != nil {
		contextDone = assertion.ctx.Done()
		if v, ok := assertion.ctx.Value("GINKGO_SPEC_CONTEXT").(contextWithAttachProgressReporter); ok {
			detach := v.AttachProgressReporter(messageGenerator)
			defer detach()
		}
	}

	if assertion.asyncType == AsyncAssertionTypeEventually {
		for {
			if err == nil && matches == desiredMatch {
				return true
			}

			if !mayChange {
				fail("No future change is possible.  Bailing out early")
				return false
			}

			select {
			case <-time.After(assertion.pollingInterval):
				v, e := assertion.pollActual()
				lock.Lock()
				value, err = v, e
				lock.Unlock()
				if err == nil {
					mayChange = assertion.matcherMayChange(matcher, value)
					matches, e = matcher.Match(value)
					lock.Lock()
					err = e
					lock.Unlock()
				}
			case <-contextDone:
				fail("Context was cancelled")
				return false
			case <-timeout:
				fail("Timed out")
				return false
			}
		}
	} else if assertion.asyncType == AsyncAssertionTypeConsistently {
		for {
			if !(err == nil && matches == desiredMatch) {
				fail("Failed")
				return false
			}

			if !mayChange {
				return true
			}

			select {
			case <-time.After(assertion.pollingInterval):
				v, e := assertion.pollActual()
				lock.Lock()
				value, err = v, e
				lock.Unlock()
				if err == nil {
					mayChange = assertion.matcherMayChange(matcher, value)
					matches, e = matcher.Match(value)
					lock.Lock()
					err = e
					lock.Unlock()
				}
			case <-contextDone:
				fail("Context was cancelled")
				return false
			case <-timeout:
				return true
			}
		}
	}

	return false
}
