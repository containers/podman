package gstruct_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Slice", func() {
	allElements := []string{"a", "b"}
	missingElements := []string{"a"}
	extraElements := []string{"a", "b", "c"}
	duplicateElements := []string{"a", "a", "b"}
	empty := []string{}
	var nils []string

	It("should strictly match all elements", func() {
		m := MatchAllElements(id, Elements{
			"b": Equal("b"),
			"a": Equal("a"),
		})
		Expect(allElements).Should(m, "should match all elements")
		Expect(missingElements).ShouldNot(m, "should fail with missing elements")
		Expect(extraElements).ShouldNot(m, "should fail with extra elements")
		Expect(duplicateElements).ShouldNot(m, "should fail with duplicate elements")
		Expect(nils).ShouldNot(m, "should fail with an uninitialized slice")

		m = MatchAllElements(id, Elements{
			"a": Equal("a"),
			"b": Equal("fail"),
		})
		Expect(allElements).ShouldNot(m, "should run nested matchers")

		m = MatchAllElements(id, Elements{})
		Expect(empty).Should(m, "should handle empty slices")
		Expect(allElements).ShouldNot(m, "should handle only empty slices")
		Expect(nils).Should(m, "should handle nil slices")
	})

	It("should ignore extra elements", func() {
		m := MatchElements(id, IgnoreExtras, Elements{
			"b": Equal("b"),
			"a": Equal("a"),
		})
		Expect(allElements).Should(m, "should match all elements")
		Expect(missingElements).ShouldNot(m, "should fail with missing elements")
		Expect(extraElements).Should(m, "should ignore extra elements")
		Expect(duplicateElements).ShouldNot(m, "should fail with duplicate elements")
		Expect(nils).ShouldNot(m, "should fail with an uninitialized slice")
	})

	It("should ignore missing elements", func() {
		m := MatchElements(id, IgnoreMissing, Elements{
			"a": Equal("a"),
			"b": Equal("b"),
		})
		Expect(allElements).Should(m, "should match all elements")
		Expect(missingElements).Should(m, "should ignore missing elements")
		Expect(extraElements).ShouldNot(m, "should fail with extra elements")
		Expect(duplicateElements).ShouldNot(m, "should fail with duplicate elements")
		Expect(nils).Should(m, "should ignore an uninitialized slice")
	})

	It("should ignore missing and extra elements", func() {
		m := MatchElements(id, IgnoreMissing|IgnoreExtras, Elements{
			"a": Equal("a"),
			"b": Equal("b"),
		})
		Expect(allElements).Should(m, "should match all elements")
		Expect(missingElements).Should(m, "should ignore missing elements")
		Expect(extraElements).Should(m, "should ignore extra elements")
		Expect(duplicateElements).ShouldNot(m, "should fail with duplicate elements")
		Expect(nils).Should(m, "should ignore an uninitialized slice")

		m = MatchElements(id, IgnoreExtras|IgnoreMissing, Elements{
			"a": Equal("a"),
			"b": Equal("fail"),
		})
		Expect(allElements).ShouldNot(m, "should run nested matchers")
	})

	Context("with elements that share a key", func() {
		nonUniqueID := func(element interface{}) string {
			return element.(string)[0:1]
		}

		allElements := []string{"a123", "a213", "b321"}
		includingBadElements := []string{"a123", "b123", "b5555"}
		extraElements := []string{"a123", "b1234", "c345"}
		missingElements := []string{"b123", "b1234", "b1345"}

		It("should strictly allow multiple matches", func() {
			m := MatchElements(nonUniqueID, AllowDuplicates, Elements{
				"a": ContainSubstring("1"),
				"b": ContainSubstring("1"),
			})
			Expect(allElements).Should(m, "should match all elements")
			Expect(includingBadElements).ShouldNot(m, "should reject if a member fails the matcher")
			Expect(extraElements).ShouldNot(m, "should reject with extra keys")
			Expect(missingElements).ShouldNot(m, "should reject with missing keys")
			Expect(nils).ShouldNot(m, "should fail with an uninitialized slice")
		})

		It("should ignore missing", func() {
			m := MatchElements(nonUniqueID, AllowDuplicates|IgnoreMissing, Elements{
				"a": ContainSubstring("1"),
				"b": ContainSubstring("1"),
			})
			Expect(allElements).Should(m, "should match all elements")
			Expect(includingBadElements).ShouldNot(m, "should reject if a member fails the matcher")
			Expect(extraElements).ShouldNot(m, "should reject with extra keys")
			Expect(missingElements).Should(m, "should allow missing keys")
			Expect(nils).Should(m, "should allow an uninitialized slice")
		})

		It("should ignore extras", func() {
			m := MatchElements(nonUniqueID, AllowDuplicates|IgnoreExtras, Elements{
				"a": ContainSubstring("1"),
				"b": ContainSubstring("1"),
			})
			Expect(allElements).Should(m, "should match all elements")
			Expect(includingBadElements).ShouldNot(m, "should reject if a member fails the matcher")
			Expect(extraElements).Should(m, "should allow extra keys")
			Expect(missingElements).ShouldNot(m, "should reject missing keys")
			Expect(nils).ShouldNot(m, "should reject an uninitialized slice")
		})

		It("should ignore missing and extras", func() {
			m := MatchElements(nonUniqueID, AllowDuplicates|IgnoreExtras|IgnoreMissing, Elements{
				"a": ContainSubstring("1"),
				"b": ContainSubstring("1"),
			})
			Expect(allElements).Should(m, "should match all elements")
			Expect(includingBadElements).ShouldNot(m, "should reject if a member fails the matcher")
			Expect(extraElements).Should(m, "should allow extra keys")
			Expect(missingElements).Should(m, "should allow missing keys")
			Expect(nils).Should(m, "should allow an uninitialized slice")
		})
	})
})

func id(element interface{}) string {
	return element.(string)
}
