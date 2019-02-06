package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BeNil", func() {
	It("should succeed when passed nil", func() {
		Expect(nil).Should(BeNil())
	})

	It("should succeed when passed a typed nil", func() {
		var a []int
		Expect(a).Should(BeNil())
	})

	It("should succeed when passing nil pointer", func() {
		var f *struct{}
		Expect(f).Should(BeNil())
	})

	It("should not succeed when not passed nil", func() {
		Expect(0).ShouldNot(BeNil())
		Expect(false).ShouldNot(BeNil())
		Expect("").ShouldNot(BeNil())
	})
})
