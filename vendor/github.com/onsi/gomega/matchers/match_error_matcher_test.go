package matchers_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

type CustomError struct {
}

func (c CustomError) Error() string {
	return "an error"
}

var _ = Describe("MatchErrorMatcher", func() {
	Context("When asserting against an error", func() {
		It("should succeed when matching with an error", func() {
			err := errors.New("an error")
			fmtErr := fmt.Errorf("an error")
			customErr := CustomError{}

			Expect(err).Should(MatchError(errors.New("an error")))
			Expect(err).ShouldNot(MatchError(errors.New("another error")))

			Expect(fmtErr).Should(MatchError(errors.New("an error")))
			Expect(customErr).Should(MatchError(CustomError{}))
		})

		It("should succeed when matching with a string", func() {
			err := errors.New("an error")
			fmtErr := fmt.Errorf("an error")
			customErr := CustomError{}

			Expect(err).Should(MatchError("an error"))
			Expect(err).ShouldNot(MatchError("another error"))

			Expect(fmtErr).Should(MatchError("an error"))
			Expect(customErr).Should(MatchError("an error"))
		})

		Context("when passed a matcher", func() {
			It("should pass if the matcher passes against the error string", func() {
				err := errors.New("error 123 abc")

				Expect(err).Should(MatchError(MatchRegexp(`\d{3}`)))
			})

			It("should fail if the matcher fails against the error string", func() {
				err := errors.New("no digits")
				Expect(err).ShouldNot(MatchError(MatchRegexp(`\d`)))
			})
		})

		It("should fail when passed anything else", func() {
			actualErr := errors.New("an error")
			_, err := (&MatchErrorMatcher{
				Expected: []byte("an error"),
			}).Match(actualErr)
			Expect(err).Should(HaveOccurred())

			_, err = (&MatchErrorMatcher{
				Expected: 3,
			}).Match(actualErr)
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("when passed nil", func() {
		It("should fail", func() {
			_, err := (&MatchErrorMatcher{
				Expected: "an error",
			}).Match(nil)
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("when passed a non-error", func() {
		It("should fail", func() {
			_, err := (&MatchErrorMatcher{
				Expected: "an error",
			}).Match("an error")
			Expect(err).Should(HaveOccurred())

			_, err = (&MatchErrorMatcher{
				Expected: "an error",
			}).Match(3)
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("when passed an error that is also a string", func() {
		It("should use it as an error", func() {
			var e mockErr = "mockErr"

			// this fails if the matcher casts e to a string before comparison
			Expect(e).Should(MatchError(e))
		})
	})
})

type mockErr string

func (m mockErr) Error() string { return string(m) }
