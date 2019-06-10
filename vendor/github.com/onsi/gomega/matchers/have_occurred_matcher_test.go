package matchers_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

type CustomErr struct {
	msg string
}

func (e *CustomErr) Error() string {
	return e.msg
}

var _ = Describe("HaveOccurred", func() {
	It("should succeed if matching an error", func() {
		Expect(errors.New("Foo")).Should(HaveOccurred())
	})

	It("should not succeed with nil", func() {
		Expect(nil).ShouldNot(HaveOccurred())
	})

	It("should only support errors and nil", func() {
		success, err := (&HaveOccurredMatcher{}).Match("foo")
		Expect(success).Should(BeFalse())
		Expect(err).Should(HaveOccurred())

		success, err = (&HaveOccurredMatcher{}).Match("")
		Expect(success).Should(BeFalse())
		Expect(err).Should(HaveOccurred())
	})

	It("doesn't support non-error type", func() {
		success, err := (&HaveOccurredMatcher{}).Match(AnyType{})
		Expect(success).Should(BeFalse())
		Expect(err).Should(MatchError("Expected an error-type.  Got:\n    <matchers_test.AnyType>: {}"))
	})

	It("doesn't support non-error pointer type", func() {
		success, err := (&HaveOccurredMatcher{}).Match(&AnyType{})
		Expect(success).Should(BeFalse())
		Expect(err).Should(MatchError(MatchRegexp(`Expected an error-type.  Got:\n    <*matchers_test.AnyType | 0x[[:xdigit:]]+>: {}`)))
	})

	It("should succeed with pointer types that conform to error interface", func() {
		err := &CustomErr{"ohai"}
		Expect(err).Should(HaveOccurred())
	})

	It("should not succeed with nil pointers to types that conform to error interface", func() {
		var err *CustomErr = nil
		Expect(err).ShouldNot(HaveOccurred())
	})
})
