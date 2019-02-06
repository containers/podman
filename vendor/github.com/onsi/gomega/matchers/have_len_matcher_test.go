package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("HaveLen", func() {
	Context("when passed a supported type", func() {
		It("should do the right thing", func() {
			Expect("").Should(HaveLen(0))
			Expect("AA").Should(HaveLen(2))

			Expect([0]int{}).Should(HaveLen(0))
			Expect([2]int{1, 2}).Should(HaveLen(2))

			Expect([]int{}).Should(HaveLen(0))
			Expect([]int{1, 2, 3}).Should(HaveLen(3))

			Expect(map[string]int{}).Should(HaveLen(0))
			Expect(map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}).Should(HaveLen(4))

			c := make(chan bool, 3)
			Expect(c).Should(HaveLen(0))
			c <- true
			c <- true
			Expect(c).Should(HaveLen(2))
		})
	})

	Context("when passed a correctly typed nil", func() {
		It("should operate succesfully on the passed in value", func() {
			var nilSlice []int
			Expect(nilSlice).Should(HaveLen(0))

			var nilMap map[int]string
			Expect(nilMap).Should(HaveLen(0))
		})
	})

	Context("when passed an unsupported type", func() {
		It("should error", func() {
			success, err := (&HaveLenMatcher{Count: 0}).Match(0)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&HaveLenMatcher{Count: 0}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})
})
