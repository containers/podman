package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("Panic", func() {
	Context("when passed something that's not a function that takes zero arguments and returns nothing", func() {
		It("should error", func() {
			success, err := (&PanicMatcher{}).Match("foo")
			Expect(success).To(BeFalse())
			Expect(err).To(HaveOccurred())

			success, err = (&PanicMatcher{}).Match(nil)
			Expect(success).To(BeFalse())
			Expect(err).To(HaveOccurred())

			success, err = (&PanicMatcher{}).Match(func(foo string) {})
			Expect(success).To(BeFalse())
			Expect(err).To(HaveOccurred())

			success, err = (&PanicMatcher{}).Match(func() string { return "bar" })
			Expect(success).To(BeFalse())
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when passed a function of the correct type", func() {
		It("should call the function and pass if the function panics", func() {
			Expect(func() { panic("ack!") }).To(Panic())
			Expect(func() {}).NotTo(Panic())
		})
	})

	Context("when assertion fails", func() {
		It("prints the object passed to Panic when negative", func() {
			failuresMessages := InterceptGomegaFailures(func() {
				Expect(func() { panic("ack!") }).NotTo(Panic())
			})
			Expect(failuresMessages).To(ConsistOf(ContainSubstring("not to panic, but panicked with\n    <string>: ack!")))
		})

		It("prints simple message when positive", func() {
			failuresMessages := InterceptGomegaFailures(func() {
				Expect(func() {}).To(Panic())
			})
			Expect(failuresMessages).To(ConsistOf(MatchRegexp("Expected\n\\s+<func\\(\\)>: .+\nto panic")))
		})
	})
})
