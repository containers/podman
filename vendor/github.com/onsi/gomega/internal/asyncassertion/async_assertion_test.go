package asyncassertion_test

import (
	"errors"
	"time"

	"github.com/onsi/gomega/internal/testingtsupport"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/internal/asyncassertion"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Async Assertion", func() {
	var (
		failureMessage string
		callerSkip     int
	)

	var fakeFailWrapper = &types.GomegaFailWrapper{
		Fail: func(message string, skip ...int) {
			failureMessage = message
			callerSkip = skip[0]
		},
		TWithHelper: testingtsupport.EmptyTWithHelper{},
	}

	BeforeEach(func() {
		failureMessage = ""
		callerSkip = 0
	})

	Describe("Eventually", func() {
		Context("the positive case", func() {
			It("should poll the function and matcher", func() {
				counter := 0
				a := New(AsyncAssertionTypeEventually, func() int {
					counter++
					return counter
				}, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)

				a.Should(BeNumerically("==", 5))
				Expect(failureMessage).Should(BeZero())
			})

			It("should continue when the matcher errors", func() {
				counter := 0
				a := New(AsyncAssertionTypeEventually, func() interface{} {
					counter++
					if counter == 5 {
						return "not-a-number" //this should cause the matcher to error
					}
					return counter
				}, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)

				a.Should(BeNumerically("==", 5), "My description %d", 2)

				Expect(failureMessage).Should(ContainSubstring("Timed out after"))
				Expect(failureMessage).Should(ContainSubstring("My description 2"))
				Expect(callerSkip).Should(Equal(4))
			})

			It("should be able to timeout", func() {
				counter := 0
				a := New(AsyncAssertionTypeEventually, func() int {
					counter++
					return counter
				}, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)

				a.Should(BeNumerically(">", 100), "My description %d", 2)

				Expect(counter).Should(BeNumerically(">", 8))
				Expect(counter).Should(BeNumerically("<=", 10))
				Expect(failureMessage).Should(ContainSubstring("Timed out after"))
				Expect(failureMessage).Should(MatchRegexp(`\<int\>: \d`), "Should pass the correct value to the matcher message formatter.")
				Expect(failureMessage).Should(ContainSubstring("My description 2"))
				Expect(callerSkip).Should(Equal(4))
			})
		})

		Context("the negative case", func() {
			It("should poll the function and matcher", func() {
				counter := 0
				a := New(AsyncAssertionTypeEventually, func() int {
					counter += 1
					return counter
				}, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)

				a.ShouldNot(BeNumerically("<", 3))

				Expect(counter).Should(Equal(3))
				Expect(failureMessage).Should(BeZero())
			})

			It("should timeout when the matcher errors", func() {
				a := New(AsyncAssertionTypeEventually, func() interface{} {
					return 0 //this should cause the matcher to error
				}, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)

				a.ShouldNot(HaveLen(0), "My description %d", 2)

				Expect(failureMessage).Should(ContainSubstring("Timed out after"))
				Expect(failureMessage).Should(ContainSubstring("Error:"))
				Expect(failureMessage).Should(ContainSubstring("My description 2"))
				Expect(callerSkip).Should(Equal(4))
			})

			It("should be able to timeout", func() {
				a := New(AsyncAssertionTypeEventually, func() int {
					return 0
				}, fakeFailWrapper, time.Duration(0.1*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)

				a.ShouldNot(Equal(0), "My description %d", 2)

				Expect(failureMessage).Should(ContainSubstring("Timed out after"))
				Expect(failureMessage).Should(ContainSubstring("<int>: 0"), "Should pass the correct value to the matcher message formatter.")
				Expect(failureMessage).Should(ContainSubstring("My description 2"))
				Expect(callerSkip).Should(Equal(4))
			})
		})

		Context("with a function that returns multiple values", func() {
			It("should eventually succeed if the additional arguments are nil", func() {
				i := 0
				Eventually(func() (int, error) {
					i++
					return i, nil
				}).Should(Equal(10))
			})

			It("should eventually timeout if the additional arguments are not nil", func() {
				i := 0
				a := New(AsyncAssertionTypeEventually, func() (int, error) {
					i++
					return i, errors.New("bam")
				}, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)
				a.Should(Equal(2))

				Expect(failureMessage).Should(ContainSubstring("Timed out after"))
				Expect(failureMessage).Should(ContainSubstring("Error:"))
				Expect(failureMessage).Should(ContainSubstring("bam"))
				Expect(callerSkip).Should(Equal(4))
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
				c := make(chan bool, 1)
				c <- true
				Eventually(c).Should(Receive())
			})
		})
	})

	Describe("Consistently", func() {
		Describe("The positive case", func() {
			Context("when the matcher consistently passes for the duration", func() {
				It("should pass", func() {
					calls := 0
					a := New(AsyncAssertionTypeConsistently, func() string {
						calls++
						return "foo"
					}, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)

					a.Should(Equal("foo"))
					Expect(calls).Should(BeNumerically(">", 8))
					Expect(calls).Should(BeNumerically("<=", 10))
					Expect(failureMessage).Should(BeZero())
				})
			})

			Context("when the matcher fails at some point", func() {
				It("should fail", func() {
					calls := 0
					a := New(AsyncAssertionTypeConsistently, func() interface{} {
						calls++
						if calls > 5 {
							return "bar"
						}
						return "foo"
					}, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)

					a.Should(Equal("foo"))
					Expect(failureMessage).Should(ContainSubstring("to equal"))
					Expect(callerSkip).Should(Equal(4))
				})
			})

			Context("when the matcher errors at some point", func() {
				It("should fail", func() {
					calls := 0
					a := New(AsyncAssertionTypeConsistently, func() interface{} {
						calls++
						if calls > 5 {
							return 3
						}
						return []int{1, 2, 3}
					}, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)

					a.Should(HaveLen(3))
					Expect(failureMessage).Should(ContainSubstring("HaveLen matcher expects"))
					Expect(callerSkip).Should(Equal(4))
				})
			})
		})

		Describe("The negative case", func() {
			Context("when the matcher consistently passes for the duration", func() {
				It("should pass", func() {
					c := make(chan bool)
					a := New(AsyncAssertionTypeConsistently, c, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)

					a.ShouldNot(Receive())
					Expect(failureMessage).Should(BeZero())
				})
			})

			Context("when the matcher fails at some point", func() {
				It("should fail", func() {
					c := make(chan bool)
					go func() {
						time.Sleep(time.Duration(100 * time.Millisecond))
						c <- true
					}()

					a := New(AsyncAssertionTypeConsistently, c, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)

					a.ShouldNot(Receive())
					Expect(failureMessage).Should(ContainSubstring("not to receive anything"))
				})
			})

			Context("when the matcher errors at some point", func() {
				It("should fail", func() {
					calls := 0
					a := New(AsyncAssertionTypeConsistently, func() interface{} {
						calls++
						return calls
					}, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)

					a.ShouldNot(BeNumerically(">", 5))
					Expect(failureMessage).Should(ContainSubstring("not to be >"))
					Expect(callerSkip).Should(Equal(4))
				})
			})
		})

		Context("with a function that returns multiple values", func() {
			It("should consistently succeed if the additional arguments are nil", func() {
				i := 2
				Consistently(func() (int, error) {
					i++
					return i, nil
				}).Should(BeNumerically(">=", 2))
			})

			It("should eventually timeout if the additional arguments are not nil", func() {
				i := 2
				a := New(AsyncAssertionTypeEventually, func() (int, error) {
					i++
					return i, errors.New("bam")
				}, fakeFailWrapper, time.Duration(0.2*float64(time.Second)), time.Duration(0.02*float64(time.Second)), 1)
				a.Should(BeNumerically(">=", 2))

				Expect(failureMessage).Should(ContainSubstring("Error:"))
				Expect(failureMessage).Should(ContainSubstring("bam"))
				Expect(callerSkip).Should(Equal(4))
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
				c := make(chan bool)
				Consistently(c).ShouldNot(Receive())
			})
		})
	})

	Context("when passed a function with the wrong # or arguments & returns", func() {
		It("should panic", func() {
			Expect(func() {
				New(AsyncAssertionTypeEventually, func() {}, fakeFailWrapper, 0, 0, 1)
			}).Should(Panic())

			Expect(func() {
				New(AsyncAssertionTypeEventually, func(a string) int { return 0 }, fakeFailWrapper, 0, 0, 1)
			}).Should(Panic())

			Expect(func() {
				New(AsyncAssertionTypeEventually, func() int { return 0 }, fakeFailWrapper, 0, 0, 1)
			}).ShouldNot(Panic())

			Expect(func() {
				New(AsyncAssertionTypeEventually, func() (int, error) { return 0, nil }, fakeFailWrapper, 0, 0, 1)
			}).ShouldNot(Panic())
		})
	})

	Describe("bailing early", func() {
		Context("when actual is a value", func() {
			It("Eventually should bail out and fail early if the matcher says to", func() {
				c := make(chan bool)
				close(c)

				t := time.Now()
				failures := InterceptGomegaFailures(func() {
					Eventually(c, 0.1).Should(Receive())
				})
				Expect(time.Since(t)).Should(BeNumerically("<", 90*time.Millisecond))

				Expect(failures).Should(HaveLen(1))
			})
		})

		Context("when actual is a function", func() {
			It("should never bail early", func() {
				c := make(chan bool)
				close(c)

				t := time.Now()
				failures := InterceptGomegaFailures(func() {
					Eventually(func() chan bool {
						return c
					}, 0.1).Should(Receive())
				})
				Expect(time.Since(t)).Should(BeNumerically(">=", 90*time.Millisecond))

				Expect(failures).Should(HaveLen(1))
			})
		})
	})
})
