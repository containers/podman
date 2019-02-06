package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("HaveKey", func() {
	var (
		stringKeys map[string]int
		intKeys    map[int]string
		objKeys    map[*myCustomType]string

		customA *myCustomType
		customB *myCustomType
	)
	BeforeEach(func() {
		stringKeys = map[string]int{"foo": 2, "bar": 3}
		intKeys = map[int]string{2: "foo", 3: "bar"}

		customA = &myCustomType{s: "a", n: 2, f: 2.3, arr: []string{"ice", "cream"}}
		customB = &myCustomType{s: "b", n: 4, f: 3.1, arr: []string{"cake"}}
		objKeys = map[*myCustomType]string{customA: "aardvark", customB: "kangaroo"}
	})

	Context("when passed a map", func() {
		It("should do the right thing", func() {
			Expect(stringKeys).Should(HaveKey("foo"))
			Expect(stringKeys).ShouldNot(HaveKey("baz"))

			Expect(intKeys).Should(HaveKey(2))
			Expect(intKeys).ShouldNot(HaveKey(4))

			Expect(objKeys).Should(HaveKey(customA))
			Expect(objKeys).Should(HaveKey(&myCustomType{s: "b", n: 4, f: 3.1, arr: []string{"cake"}}))
			Expect(objKeys).ShouldNot(HaveKey(&myCustomType{s: "b", n: 4, f: 3.1, arr: []string{"apple", "pie"}}))
		})
	})

	Context("when passed a correctly typed nil", func() {
		It("should operate succesfully on the passed in value", func() {
			var nilMap map[int]string
			Expect(nilMap).ShouldNot(HaveKey("foo"))
		})
	})

	Context("when the passed in key is actually a matcher", func() {
		It("should pass each element through the matcher", func() {
			Expect(stringKeys).Should(HaveKey(ContainSubstring("oo")))
			Expect(stringKeys).ShouldNot(HaveKey(ContainSubstring("foobar")))
		})

		It("should fail if the matcher ever fails", func() {
			actual := map[int]string{1: "a", 3: "b", 2: "c"}
			success, err := (&HaveKeyMatcher{Key: ContainSubstring("ar")}).Match(actual)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("when passed something that is not a map", func() {
		It("should error", func() {
			success, err := (&HaveKeyMatcher{Key: "foo"}).Match([]string{"foo"})
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&HaveKeyMatcher{Key: "foo"}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})
})
