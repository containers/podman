package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("ContainElement", func() {
	Context("when passed a supported type", func() {
		Context("and expecting a non-matcher", func() {
			It("should do the right thing", func() {
				Expect([2]int{1, 2}).Should(ContainElement(2))
				Expect([2]int{1, 2}).ShouldNot(ContainElement(3))

				Expect([]int{1, 2}).Should(ContainElement(2))
				Expect([]int{1, 2}).ShouldNot(ContainElement(3))

				Expect(map[string]int{"foo": 1, "bar": 2}).Should(ContainElement(2))
				Expect(map[int]int{3: 1, 4: 2}).ShouldNot(ContainElement(3))

				arr := make([]myCustomType, 2)
				arr[0] = myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}
				arr[1] = myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "c"}}
				Expect(arr).Should(ContainElement(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}))
				Expect(arr).ShouldNot(ContainElement(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"b", "c"}}))
			})
		})

		Context("and expecting a matcher", func() {
			It("should pass each element through the matcher", func() {
				Expect([]int{1, 2, 3}).Should(ContainElement(BeNumerically(">=", 3)))
				Expect([]int{1, 2, 3}).ShouldNot(ContainElement(BeNumerically(">", 3)))
				Expect(map[string]int{"foo": 1, "bar": 2}).Should(ContainElement(BeNumerically(">=", 2)))
				Expect(map[string]int{"foo": 1, "bar": 2}).ShouldNot(ContainElement(BeNumerically(">", 2)))
			})

			It("should power through even if the matcher ever fails", func() {
				Expect([]interface{}{1, 2, "3", 4}).Should(ContainElement(BeNumerically(">=", 3)))
			})

			It("should fail if the matcher fails", func() {
				actual := []interface{}{1, 2, "3", "4"}
				success, err := (&ContainElementMatcher{Element: BeNumerically(">=", 3)}).Match(actual)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})
		})
	})

	Context("when passed a correctly typed nil", func() {
		It("should operate succesfully on the passed in value", func() {
			var nilSlice []int
			Expect(nilSlice).ShouldNot(ContainElement(1))

			var nilMap map[int]string
			Expect(nilMap).ShouldNot(ContainElement("foo"))
		})
	})

	Context("when passed an unsupported type", func() {
		It("should error", func() {
			success, err := (&ContainElementMatcher{Element: 0}).Match(0)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&ContainElementMatcher{Element: 0}).Match("abc")
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&ContainElementMatcher{Element: 0}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})
})
