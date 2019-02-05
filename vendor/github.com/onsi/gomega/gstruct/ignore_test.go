package gstruct_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Ignore", func() {
	It("should always succeed", func() {
		Expect(nil).Should(Ignore())
		Expect(struct{}{}).Should(Ignore())
		Expect(0).Should(Ignore())
		Expect(false).Should(Ignore())
	})

	It("should always fail", func() {
		Expect(nil).ShouldNot(Reject())
		Expect(struct{}{}).ShouldNot(Reject())
		Expect(1).ShouldNot(Reject())
		Expect(true).ShouldNot(Reject())
	})
})
