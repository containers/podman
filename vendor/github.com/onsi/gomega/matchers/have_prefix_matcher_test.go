package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("HavePrefixMatcher", func() {
	Context("when actual is a string", func() {
		It("should match a string prefix", func() {
			Expect("Ab").Should(HavePrefix("A"))
			Expect("A").ShouldNot(HavePrefix("Ab"))
		})
	})

	Context("when the matcher is called with multiple arguments", func() {
		It("should pass the string and arguments to sprintf", func() {
			Expect("C3PO").Should(HavePrefix("C%dP", 3))
		})
	})

	Context("when actual is a stringer", func() {
		It("should call the stringer and match against the returned string", func() {
			Expect(&myStringer{a: "Ab"}).Should(HavePrefix("A"))
		})
	})

	Context("when actual is neither a string nor a stringer", func() {
		It("should error", func() {
			success, err := (&HavePrefixMatcher{Prefix: "2"}).Match(2)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})
})
