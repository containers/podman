package matchers_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("BeIdenticalTo", func() {
	Context("when asserting that nil equals nil", func() {
		It("should error", func() {
			success, err := (&BeIdenticalToMatcher{Expected: nil}).Match(nil)

			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	It("should treat the same pointer to a struct as identical", func() {
		mySpecialStruct := myCustomType{}
		Expect(&mySpecialStruct).Should(BeIdenticalTo(&mySpecialStruct))
		Expect(&myCustomType{}).ShouldNot(BeIdenticalTo(&mySpecialStruct))
	})

	It("should be strict about types", func() {
		Expect(5).ShouldNot(BeIdenticalTo("5"))
		Expect(5).ShouldNot(BeIdenticalTo(5.0))
		Expect(5).ShouldNot(BeIdenticalTo(3))
	})

	It("should treat primtives as identical", func() {
		Expect("5").Should(BeIdenticalTo("5"))
		Expect("5").ShouldNot(BeIdenticalTo("55"))

		Expect(5.55).Should(BeIdenticalTo(5.55))
		Expect(5.55).ShouldNot(BeIdenticalTo(6.66))

		Expect(5).Should(BeIdenticalTo(5))
		Expect(5).ShouldNot(BeIdenticalTo(55))
	})

	It("should treat the same pointers to a slice as identical", func() {
		mySlice := []int{1, 2}
		Expect(&mySlice).Should(BeIdenticalTo(&mySlice))
		Expect(&mySlice).ShouldNot(BeIdenticalTo(&[]int{1, 2}))
	})

	It("should treat the same pointers to a map as identical", func() {
		myMap := map[string]string{"a": "b", "c": "d"}
		Expect(&myMap).Should(BeIdenticalTo(&myMap))
		Expect(myMap).ShouldNot(BeIdenticalTo(map[string]string{"a": "b", "c": "d"}))
	})

	It("should treat the same pointers to an error as identical", func() {
		myError := errors.New("foo")
		Expect(&myError).Should(BeIdenticalTo(&myError))
		Expect(errors.New("foo")).ShouldNot(BeIdenticalTo(errors.New("bar")))
	})
})
