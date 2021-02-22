package integration

import (
	"os"
	"sort"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman diff", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman diff of image", func() {
		session := podmanTest.Podman([]string{"diff", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 0))
	})

	It("podman container diff of image", func() {
		session := podmanTest.Podman([]string{"container", "diff", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 0))
	})

	It("podman diff bogus image", func() {
		session := podmanTest.Podman([]string{"diff", "1234"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman diff image with json output", func() {
		session := podmanTest.Podman([]string{"diff", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman diff container and committed image", func() {
		session := podmanTest.Podman([]string{"run", "--name=diff-test", ALPINE, "touch", "/tmp/diff-test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"diff", "diff-test"})
		session.WaitWithDefaultTimeout()
		containerDiff := session.OutputToStringArray()
		sort.Strings(containerDiff)
		Expect(session.LineInOutputContains("C /tmp")).To(BeTrue())
		Expect(session.LineInOutputContains("A /tmp/diff-test")).To(BeTrue())
		session = podmanTest.Podman([]string{"commit", "diff-test", "diff-test-img"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"diff", "diff-test-img"})
		session.WaitWithDefaultTimeout()
		imageDiff := session.OutputToStringArray()
		sort.Strings(imageDiff)
		Expect(imageDiff).To(Equal(containerDiff))
	})

	It("podman diff latest container", func() {
		session := podmanTest.Podman([]string{"run", "--name", "diff-test", ALPINE, "touch", "/tmp/diff-test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"diff", "diff-test"})
		session.WaitWithDefaultTimeout()
		containerDiff := session.OutputToStringArray()
		sort.Strings(containerDiff)
		Expect(session.LineInOutputContains("C /tmp")).To(BeTrue())
		Expect(session.LineInOutputContains("A /tmp/diff-test")).To(BeTrue())
		Expect(session.ExitCode()).To(Equal(0))
	})
})
