package assertion_test

import (
	"errors"

	"github.com/onsi/gomega/internal/testingtsupport"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/internal/assertion"
	"github.com/onsi/gomega/internal/fakematcher"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Assertion", func() {
	var (
		a                 *Assertion
		failureMessage    string
		failureCallerSkip int
		matcher           *fakematcher.FakeMatcher
	)

	input := "The thing I'm testing"

	var fakeFailWrapper = &types.GomegaFailWrapper{
		Fail: func(message string, callerSkip ...int) {
			failureMessage = message
			if len(callerSkip) == 1 {
				failureCallerSkip = callerSkip[0]
			}
		},
		TWithHelper: testingtsupport.EmptyTWithHelper{},
	}

	BeforeEach(func() {
		matcher = &fakematcher.FakeMatcher{}
		failureMessage = ""
		failureCallerSkip = 0
		a = New(input, fakeFailWrapper, 1)
	})

	Context("when called", func() {
		It("should pass the provided input value to the matcher", func() {
			a.Should(matcher)

			Expect(matcher.ReceivedActual).Should(Equal(input))
			matcher.ReceivedActual = ""

			a.ShouldNot(matcher)

			Expect(matcher.ReceivedActual).Should(Equal(input))
			matcher.ReceivedActual = ""

			a.To(matcher)

			Expect(matcher.ReceivedActual).Should(Equal(input))
			matcher.ReceivedActual = ""

			a.ToNot(matcher)

			Expect(matcher.ReceivedActual).Should(Equal(input))
			matcher.ReceivedActual = ""

			a.NotTo(matcher)

			Expect(matcher.ReceivedActual).Should(Equal(input))
		})
	})

	Context("when the matcher succeeds", func() {
		BeforeEach(func() {
			matcher.MatchesToReturn = true
			matcher.ErrToReturn = nil
		})

		Context("and a positive assertion is being made", func() {
			It("should not call the failure callback", func() {
				a.Should(matcher)
				Expect(failureMessage).Should(Equal(""))
			})

			It("should be true", func() {
				Expect(a.Should(matcher)).Should(BeTrue())
			})
		})

		Context("and a negative assertion is being made", func() {
			It("should call the failure callback", func() {
				a.ShouldNot(matcher)
				Expect(failureMessage).Should(Equal("negative: The thing I'm testing"))
				Expect(failureCallerSkip).Should(Equal(3))
			})

			It("should be false", func() {
				Expect(a.ShouldNot(matcher)).Should(BeFalse())
			})
		})
	})

	Context("when the matcher fails", func() {
		BeforeEach(func() {
			matcher.MatchesToReturn = false
			matcher.ErrToReturn = nil
		})

		Context("and a positive assertion is being made", func() {
			It("should call the failure callback", func() {
				a.Should(matcher)
				Expect(failureMessage).Should(Equal("positive: The thing I'm testing"))
				Expect(failureCallerSkip).Should(Equal(3))
			})

			It("should be false", func() {
				Expect(a.Should(matcher)).Should(BeFalse())
			})
		})

		Context("and a negative assertion is being made", func() {
			It("should not call the failure callback", func() {
				a.ShouldNot(matcher)
				Expect(failureMessage).Should(Equal(""))
			})

			It("should be true", func() {
				Expect(a.ShouldNot(matcher)).Should(BeTrue())
			})
		})
	})

	Context("When reporting a failure", func() {
		BeforeEach(func() {
			matcher.MatchesToReturn = false
			matcher.ErrToReturn = nil
		})

		Context("and there is an optional description", func() {
			It("should append the description to the failure message", func() {
				a.Should(matcher, "A description")
				Expect(failureMessage).Should(Equal("A description\npositive: The thing I'm testing"))
				Expect(failureCallerSkip).Should(Equal(3))
			})
		})

		Context("and there are multiple arguments to the optional description", func() {
			It("should append the formatted description to the failure message", func() {
				a.Should(matcher, "A description of [%d]", 3)
				Expect(failureMessage).Should(Equal("A description of [3]\npositive: The thing I'm testing"))
				Expect(failureCallerSkip).Should(Equal(3))
			})
		})
	})

	Context("When the matcher returns an error", func() {
		BeforeEach(func() {
			matcher.ErrToReturn = errors.New("Kaboom!")
		})

		Context("and a positive assertion is being made", func() {
			It("should call the failure callback", func() {
				matcher.MatchesToReturn = true
				a.Should(matcher)
				Expect(failureMessage).Should(Equal("Kaboom!"))
				Expect(failureCallerSkip).Should(Equal(3))
			})
		})

		Context("and a negative assertion is being made", func() {
			It("should call the failure callback", func() {
				matcher.MatchesToReturn = false
				a.ShouldNot(matcher)
				Expect(failureMessage).Should(Equal("Kaboom!"))
				Expect(failureCallerSkip).Should(Equal(3))
			})
		})

		It("should always be false", func() {
			Expect(a.Should(matcher)).Should(BeFalse())
			Expect(a.ShouldNot(matcher)).Should(BeFalse())
		})
	})

	Context("when there are extra parameters", func() {
		It("(a simple example)", func() {
			Expect(func() (string, int, error) {
				return "foo", 0, nil
			}()).Should(Equal("foo"))
		})

		Context("when the parameters are all nil or zero", func() {
			It("should invoke the matcher", func() {
				matcher.MatchesToReturn = true
				matcher.ErrToReturn = nil

				var typedNil []string
				a = New(input, fakeFailWrapper, 1, 0, nil, typedNil)

				result := a.Should(matcher)
				Expect(result).Should(BeTrue())
				Expect(matcher.ReceivedActual).Should(Equal(input))

				Expect(failureMessage).Should(BeZero())
			})
		})

		Context("when any of the parameters are not nil or zero", func() {
			It("should call the failure callback", func() {
				matcher.MatchesToReturn = false
				matcher.ErrToReturn = nil

				a = New(input, fakeFailWrapper, 1, errors.New("foo"))
				result := a.Should(matcher)
				Expect(result).Should(BeFalse())
				Expect(matcher.ReceivedActual).Should(BeZero(), "The matcher doesn't even get called")
				Expect(failureMessage).Should(ContainSubstring("foo"))
				failureMessage = ""

				a = New(input, fakeFailWrapper, 1, nil, 1)
				result = a.ShouldNot(matcher)
				Expect(result).Should(BeFalse())
				Expect(failureMessage).Should(ContainSubstring("1"))
				failureMessage = ""

				a = New(input, fakeFailWrapper, 1, nil, 0, []string{"foo"})
				result = a.To(matcher)
				Expect(result).Should(BeFalse())
				Expect(failureMessage).Should(ContainSubstring("foo"))
				failureMessage = ""

				a = New(input, fakeFailWrapper, 1, nil, 0, []string{"foo"})
				result = a.ToNot(matcher)
				Expect(result).Should(BeFalse())
				Expect(failureMessage).Should(ContainSubstring("foo"))
				failureMessage = ""

				a = New(input, fakeFailWrapper, 1, nil, 0, []string{"foo"})
				result = a.NotTo(matcher)
				Expect(result).Should(BeFalse())
				Expect(failureMessage).Should(ContainSubstring("foo"))
				Expect(failureCallerSkip).Should(Equal(3))
			})
		})
	})

	Context("Making an assertion without a registered fail handler", func() {
		It("should panic", func() {
			defer func() {
				e := recover()
				RegisterFailHandler(Fail)
				if e == nil {
					Fail("expected a panic to have occurred")
				}
			}()

			RegisterFailHandler(nil)
			Expect(true).Should(BeTrue())
		})
	})
})
