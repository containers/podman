package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("BeEquivalentTo", func() {
	Context("when asserting that nil is equivalent to nil", func() {
		It("should error", func() {
			success, err := (&BeEquivalentToMatcher{Expected: nil}).Match(nil)

			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("When asserting on nil", func() {
		It("should do the right thing", func() {
			Expect("foo").ShouldNot(BeEquivalentTo(nil))
			Expect(nil).ShouldNot(BeEquivalentTo(3))
			Expect([]int{1, 2}).ShouldNot(BeEquivalentTo(nil))
		})
	})

	Context("When asserting on type aliases", func() {
		It("should the right thing", func() {
			Expect(StringAlias("foo")).Should(BeEquivalentTo("foo"))
			Expect("foo").Should(BeEquivalentTo(StringAlias("foo")))
			Expect(StringAlias("foo")).ShouldNot(BeEquivalentTo("bar"))
			Expect("foo").ShouldNot(BeEquivalentTo(StringAlias("bar")))
		})
	})

	Context("When asserting on numbers", func() {
		It("should convert actual to expected and do the right thing", func() {
			Expect(5).Should(BeEquivalentTo(5))
			Expect(5.0).Should(BeEquivalentTo(5.0))
			Expect(5).Should(BeEquivalentTo(5.0))

			Expect(5).ShouldNot(BeEquivalentTo("5"))
			Expect(5).ShouldNot(BeEquivalentTo(3))

			//Here be dragons!
			Expect(5.1).Should(BeEquivalentTo(5))
			Expect(5).ShouldNot(BeEquivalentTo(5.1))
		})
	})
})
