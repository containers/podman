package debug_parallel_fixture_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDebugParallelFixture(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DebugParallelFixture Suite")
}
