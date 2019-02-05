package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("BeNumerically", func() {
	Context("when passed a number", func() {
		It("should support ==", func() {
			Expect(uint32(5)).Should(BeNumerically("==", 5))
			Expect(float64(5.0)).Should(BeNumerically("==", 5))
			Expect(int8(5)).Should(BeNumerically("==", 5))
		})

		It("should not have false positives", func() {
			Expect(5.1).ShouldNot(BeNumerically("==", 5))
			Expect(5).ShouldNot(BeNumerically("==", 5.1))
		})

		It("should support >", func() {
			Expect(uint32(5)).Should(BeNumerically(">", 4))
			Expect(float64(5.0)).Should(BeNumerically(">", 4.9))
			Expect(int8(5)).Should(BeNumerically(">", 4))

			Expect(uint32(5)).ShouldNot(BeNumerically(">", 5))
			Expect(float64(5.0)).ShouldNot(BeNumerically(">", 5.0))
			Expect(int8(5)).ShouldNot(BeNumerically(">", 5))
		})

		It("should support <", func() {
			Expect(uint32(5)).Should(BeNumerically("<", 6))
			Expect(float64(5.0)).Should(BeNumerically("<", 5.1))
			Expect(int8(5)).Should(BeNumerically("<", 6))

			Expect(uint32(5)).ShouldNot(BeNumerically("<", 5))
			Expect(float64(5.0)).ShouldNot(BeNumerically("<", 5.0))
			Expect(int8(5)).ShouldNot(BeNumerically("<", 5))
		})

		It("should support >=", func() {
			Expect(uint32(5)).Should(BeNumerically(">=", 4))
			Expect(float64(5.0)).Should(BeNumerically(">=", 4.9))
			Expect(int8(5)).Should(BeNumerically(">=", 4))

			Expect(uint32(5)).Should(BeNumerically(">=", 5))
			Expect(float64(5.0)).Should(BeNumerically(">=", 5.0))
			Expect(int8(5)).Should(BeNumerically(">=", 5))

			Expect(uint32(5)).ShouldNot(BeNumerically(">=", 6))
			Expect(float64(5.0)).ShouldNot(BeNumerically(">=", 5.1))
			Expect(int8(5)).ShouldNot(BeNumerically(">=", 6))
		})

		It("should support <=", func() {
			Expect(uint32(5)).Should(BeNumerically("<=", 6))
			Expect(float64(5.0)).Should(BeNumerically("<=", 5.1))
			Expect(int8(5)).Should(BeNumerically("<=", 6))

			Expect(uint32(5)).Should(BeNumerically("<=", 5))
			Expect(float64(5.0)).Should(BeNumerically("<=", 5.0))
			Expect(int8(5)).Should(BeNumerically("<=", 5))

			Expect(uint32(5)).ShouldNot(BeNumerically("<=", 4))
			Expect(float64(5.0)).ShouldNot(BeNumerically("<=", 4.9))
			Expect(int8(5)).Should(BeNumerically("<=", 5))
		})

		Context("when passed ~", func() {
			Context("when passed a float", func() {
				Context("and there is no precision parameter", func() {
					It("should default to 1e-8", func() {
						Expect(5.00000001).Should(BeNumerically("~", 5.00000002))
						Expect(5.00000001).ShouldNot(BeNumerically("~", 5.0000001))
					})

					It("should show failure message", func() {
						actual := BeNumerically("~", 4.98).FailureMessage(123)
						expected := "Expected\n    <int>: 123\nto be ~\n    <float64>: 4.98"
						Expect(actual).To(Equal(expected))
					})

					It("should show negated failure message", func() {
						actual := BeNumerically("~", 4.98).NegatedFailureMessage(123)
						expected := "Expected\n    <int>: 123\nnot to be ~\n    <float64>: 4.98"
						Expect(actual).To(Equal(expected))
					})
				})

				Context("and there is a precision parameter", func() {
					It("should use the precision parameter", func() {
						Expect(5.1).Should(BeNumerically("~", 5.19, 0.1))
						Expect(5.1).Should(BeNumerically("~", 5.01, 0.1))
						Expect(5.1).ShouldNot(BeNumerically("~", 5.22, 0.1))
						Expect(5.1).ShouldNot(BeNumerically("~", 4.98, 0.1))
					})

					It("should show precision in failure message", func() {
						actual := BeNumerically("~", 4.98, 0.1).FailureMessage(123)
						expected := "Expected\n    <int>: 123\nto be within 0.1 of ~\n    <float64>: 4.98"
						Expect(actual).To(Equal(expected))
					})

					It("should show precision in negated failure message", func() {
						actual := BeNumerically("~", 4.98, 0.1).NegatedFailureMessage(123)
						expected := "Expected\n    <int>: 123\nnot to be within 0.1 of ~\n    <float64>: 4.98"
						Expect(actual).To(Equal(expected))
					})
				})
			})

			Context("when passed an int/uint", func() {
				Context("and there is no precision parameter", func() {
					It("should just do strict equality", func() {
						Expect(5).Should(BeNumerically("~", 5))
						Expect(5).ShouldNot(BeNumerically("~", 6))
						Expect(uint(5)).ShouldNot(BeNumerically("~", 6))
					})
				})

				Context("and there is a precision parameter", func() {
					It("should use precision paramter", func() {
						Expect(5).Should(BeNumerically("~", 6, 2))
						Expect(5).ShouldNot(BeNumerically("~", 8, 2))
						Expect(uint(5)).Should(BeNumerically("~", 6, 1))
					})
				})
			})
		})
	})

	Context("when passed a non-number", func() {
		It("should error", func() {
			success, err := (&BeNumericallyMatcher{Comparator: "==", CompareTo: []interface{}{5}}).Match("foo")
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&BeNumericallyMatcher{Comparator: "=="}).Match(5)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&BeNumericallyMatcher{Comparator: "~", CompareTo: []interface{}{3.0, "foo"}}).Match(5.0)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&BeNumericallyMatcher{Comparator: "==", CompareTo: []interface{}{"bar"}}).Match(5)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&BeNumericallyMatcher{Comparator: "==", CompareTo: []interface{}{"bar"}}).Match("foo")
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&BeNumericallyMatcher{Comparator: "==", CompareTo: []interface{}{nil}}).Match(0)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&BeNumericallyMatcher{Comparator: "==", CompareTo: []interface{}{0}}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("when passed an unsupported comparator", func() {
		It("should error", func() {
			success, err := (&BeNumericallyMatcher{Comparator: "!=", CompareTo: []interface{}{5}}).Match(4)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})
})
