package framework

import (
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestFramework is used to support commonly used test features
type TestFramework struct {
	setup     func(*TestFramework) error
	teardown  func(*TestFramework) error
	TestError error
}

// NewTestFramework creates a new test framework instance for a given `setup`
// and `teardown` function
func NewTestFramework(
	setup func(*TestFramework) error,
	teardown func(*TestFramework) error,
) *TestFramework {
	return &TestFramework{
		setup,
		teardown,
		fmt.Errorf("error"),
	}
}

// NilFn is a convenience function which simply does nothing
func NilFunc(f *TestFramework) error {
	return nil
}

// Setup is the global initialization function which runs before each test
// suite
func (t *TestFramework) Setup() {
	// Global initialization for the whole framework goes in here

	// Set up the actual test suite
	gomega.Expect(t.setup(t)).To(gomega.Succeed())
}

// Teardown is the global deinitialization function which runs after each test
// suite
func (t *TestFramework) Teardown() {
	// Global deinitialization for the whole framework goes in here

	// Teardown the actual test suite
	gomega.Expect(t.teardown(t)).To(gomega.Succeed())
}

// Describe is a convenience wrapper around the `ginkgo.Describe` function
func (t *TestFramework) Describe(text string, body func()) bool {
	return ginkgo.Describe("libpod: "+text, body)
}
