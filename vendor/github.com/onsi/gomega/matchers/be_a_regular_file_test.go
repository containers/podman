package matchers_test

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("BeARegularFileMatcher", func() {
	Context("when passed a string", func() {
		It("should do the right thing", func() {
			Expect("/dne/test").ShouldNot(BeARegularFile())

			tmpFile, err := ioutil.TempFile("", "gomega-test-tempfile")
			Expect(err).ShouldNot(HaveOccurred())
			defer os.Remove(tmpFile.Name())
			Expect(tmpFile.Name()).Should(BeARegularFile())

			tmpDir, err := ioutil.TempDir("", "gomega-test-tempdir")
			Expect(err).ShouldNot(HaveOccurred())
			defer os.Remove(tmpDir)
			Expect(tmpDir).ShouldNot(BeARegularFile())
		})
	})

	Context("when passed something else", func() {
		It("should error", func() {
			success, err := (&BeARegularFileMatcher{}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&BeARegularFileMatcher{}).Match(true)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})
})
