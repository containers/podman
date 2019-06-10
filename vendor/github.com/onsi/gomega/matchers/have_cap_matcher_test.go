package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("HaveCap", func() {
	Context("when passed a supported type", func() {
		It("should do the right thing", func() {
			Expect([0]int{}).Should(HaveCap(0))
			Expect([2]int{1}).Should(HaveCap(2))

			Expect([]int{}).Should(HaveCap(0))
			Expect([]int{1, 2, 3, 4, 5}[:2]).Should(HaveCap(5))
			Expect(make([]int, 0, 5)).Should(HaveCap(5))

			c := make(chan bool, 3)
			Expect(c).Should(HaveCap(3))
			c <- true
			c <- true
			Expect(c).Should(HaveCap(3))

			Expect(make(chan bool)).Should(HaveCap(0))
		})
	})

	Context("when passed a correctly typed nil", func() {
		It("should operate succesfully on the passed in value", func() {
			var nilSlice []int
			Expect(nilSlice).Should(HaveCap(0))

			var nilChan chan int
			Expect(nilChan).Should(HaveCap(0))
		})
	})

	Context("when passed an unsupported type", func() {
		It("should error", func() {
			success, err := (&HaveCapMatcher{Count: 0}).Match(0)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&HaveCapMatcher{Count: 0}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})
})
