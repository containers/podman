package testrunner_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/ginkgo/testrunner"
	"github.com/onsi/ginkgo/ginkgo/testsuite"
	. "github.com/onsi/gomega"
)

func strAddr(s string) interface{} {
	return &s
}

func boolAddr(s bool) interface{} {
	return &s
}

func intAddr(s int) interface{} {
	return &s
}

var _ = Describe("TestRunner", func() {
	It("should pass through go opts", func() {
		//var opts map[string]interface{}
		opts := map[string]interface{}{
			"asmflags":         strAddr("a"),
			"pkgdir":           strAddr("b"),
			"gcflags":          strAddr("c"),
			"covermode":        strAddr(""),
			"coverpkg":         strAddr(""),
			"cover":            boolAddr(false),
			"blockprofilerate": intAddr(100),
		}
		tr := testrunner.New(testsuite.TestSuite{}, 1, false, 0, opts, []string{})

		args := tr.BuildArgs(".")
		// Remove the "-i" argument; This is discarded in Golang 1.10+.
		if args[2] == "-i" {
			args = append(args[0:2], args[3:]...)
		}
		Ω(args).Should(Equal([]string{
			"test",
			"-c",
			"-o",
			".",
			"",
			"-blockprofilerate=100",
			"-asmflags=a",
			"-pkgdir=b",
			"-gcflags=c",
		}))
	})
})

func TestTestRunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Runner Suite")
}
