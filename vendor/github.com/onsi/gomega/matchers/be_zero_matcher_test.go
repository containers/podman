package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BeZero", func() {
	It("succeeds for zero values for its type", func() {
		Expect(nil).Should(BeZero())

		Expect("").Should(BeZero())
		Expect(" ").ShouldNot(BeZero())

		Expect(0).Should(BeZero())
		Expect(1).ShouldNot(BeZero())

		Expect(0.0).Should(BeZero())
		Expect(0.1).ShouldNot(BeZero())

		// Expect([]int{}).Should(BeZero())
		Expect([]int{1}).ShouldNot(BeZero())

		// Expect(map[string]int{}).Should(BeZero())
		Expect(map[string]int{"a": 1}).ShouldNot(BeZero())

		Expect(myCustomType{}).Should(BeZero())
		Expect(myCustomType{s: "a"}).ShouldNot(BeZero())
	})

	It("builds failure message", func() {
		actual := BeZero().FailureMessage(123)
		Expect(actual).To(Equal("Expected\n    <int>: 123\nto be zero-valued"))
	})

	It("builds negated failure message", func() {
		actual := BeZero().NegatedFailureMessage(123)
		Expect(actual).To(Equal("Expected\n    <int>: 123\nnot to be zero-valued"))
	})
})
