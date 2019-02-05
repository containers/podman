package testingtsupport_test

import (
	"regexp"
	"time"

	"github.com/onsi/gomega/internal/testingtsupport"

	. "github.com/onsi/gomega"

	"fmt"
	"testing"
)

func TestTestingT(t *testing.T) {
	RegisterTestingT(t)
	Ω(true).Should(BeTrue())
}

type FakeTWithHelper struct {
	LastFatal string
}

func (f *FakeTWithHelper) Fatalf(format string, args ...interface{}) {
	f.LastFatal = fmt.Sprintf(format, args...)
}

func TestGomegaWithTWithoutHelper(t *testing.T) {
	g := NewGomegaWithT(t)

	testingtsupport.StackTracePruneRE = regexp.MustCompile(`\/ginkgo\/`)

	f := &FakeTWithHelper{}
	testG := NewGomegaWithT(f)

	testG.Expect("foo").To(Equal("foo"))
	g.Expect(f.LastFatal).To(BeZero())

	testG.Expect("foo").To(Equal("bar"))
	g.Expect(f.LastFatal).To(ContainSubstring("<string>: foo"))
	g.Expect(f.LastFatal).To(ContainSubstring("testingtsupport_test"), "It should include a stacktrace")

	testG.Eventually("foo2", time.Millisecond).Should(Equal("bar"))
	g.Expect(f.LastFatal).To(ContainSubstring("<string>: foo2"))

	testG.Consistently("foo3", time.Millisecond).Should(Equal("bar"))
	g.Expect(f.LastFatal).To(ContainSubstring("<string>: foo3"))
}

type FakeTWithoutHelper struct {
	LastFatal   string
	HelperCount int
}

func (f *FakeTWithoutHelper) Fatalf(format string, args ...interface{}) {
	f.LastFatal = fmt.Sprintf(format, args...)
}

func (f *FakeTWithoutHelper) Helper() {
	f.HelperCount += 1
}

func (f *FakeTWithoutHelper) ResetHelper() {
	f.HelperCount = 0
}

func TestGomegaWithTWithHelper(t *testing.T) {
	g := NewGomegaWithT(t)

	f := &FakeTWithoutHelper{}
	testG := NewGomegaWithT(f)

	testG.Expect("foo").To(Equal("foo"))
	g.Expect(f.LastFatal).To(BeZero())
	g.Expect(f.HelperCount).To(BeNumerically(">", 0))
	f.ResetHelper()

	testG.Expect("foo").To(Equal("bar"))
	g.Expect(f.LastFatal).To(ContainSubstring("<string>: foo"))
	g.Expect(f.LastFatal).NotTo(ContainSubstring("testingtsupport_test"), "It should _not_ include a stacktrace")
	g.Expect(f.HelperCount).To(BeNumerically(">", 0))
	f.ResetHelper()

	testG.Eventually("foo2", time.Millisecond).Should(Equal("bar"))
	g.Expect(f.LastFatal).To(ContainSubstring("<string>: foo2"))
	g.Expect(f.HelperCount).To(BeNumerically(">", 0))
	f.ResetHelper()

	testG.Consistently("foo3", time.Millisecond).Should(Equal("bar"))
	g.Expect(f.LastFatal).To(ContainSubstring("<string>: foo3"))
	g.Expect(f.HelperCount).To(BeNumerically(">", 0))
}
