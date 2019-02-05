package no_test_fn_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/integration/_fixtures/no_test_fn"
	. "github.com/onsi/gomega"
)

var _ = Describe("NoTestFn", func() {
	It("should proxy strings", func() {
		Ω(StringIdentity("foo")).Should(Equal("foo"))
	})
})
