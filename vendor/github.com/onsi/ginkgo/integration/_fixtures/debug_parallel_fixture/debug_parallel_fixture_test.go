package debug_parallel_fixture_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("DebugParallelFixture", func() {
	It("emits output to a file", func() {
		for i := 0; i < 10; i += 1 {
			fmt.Printf("StdOut %d\n", i)
			GinkgoWriter.Write([]byte(fmt.Sprintf("GinkgoWriter %d\n", i)))
		}
		time.Sleep(time.Second)
	})
})
