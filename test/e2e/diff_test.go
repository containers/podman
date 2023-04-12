package integration

import (
	"fmt"
	"os"
	"sort"

	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman diff of image", func() {
		session := podmanTest.Podman([]string{"diff", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())
	})

	It("podman diff bogus image", func() {
		session := podmanTest.Podman([]string{"diff", "1234"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman diff image with json output", func() {
		session := podmanTest.Podman([]string{"diff", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(BeValidJSON())
	})

	It("podman diff container and committed image", func() {
		session := podmanTest.Podman([]string{"run", "--name=diff-test", ALPINE, "touch", "/tmp/diff-test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"diff", "diff-test"})
		session.WaitWithDefaultTimeout()
		containerDiff := session.OutputToStringArray()
		sort.Strings(containerDiff)
		Expect(session.OutputToString()).To(ContainSubstring("C /tmp"))
		Expect(session.OutputToString()).To(ContainSubstring("A /tmp/diff-test"))
		session = podmanTest.Podman([]string{"commit", "diff-test", "diff-test-img"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"diff", "diff-test-img"})
		session.WaitWithDefaultTimeout()
		imageDiff := session.OutputToStringArray()
		sort.Strings(imageDiff)
		Expect(imageDiff).To(Equal(containerDiff))
	})

	It("podman diff latest container", func() {
		session := podmanTest.Podman([]string{"run", "--name", "diff-test", ALPINE, "touch", "/tmp/diff-test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		if !IsRemote() {
			session = podmanTest.Podman([]string{"diff", "-l"})
		} else {
			session = podmanTest.Podman([]string{"diff", "diff-test"})
		}
		session.WaitWithDefaultTimeout()
		containerDiff := session.OutputToStringArray()
		sort.Strings(containerDiff)
		Expect(session.OutputToString()).To(ContainSubstring("C /tmp"))
		Expect(session.OutputToString()).To(ContainSubstring("A /tmp/diff-test"))
		Expect(session).Should(Exit(0))
	})

	It("podman image diff", func() {
		file1 := "/" + stringid.GenerateRandomID()
		file2 := "/" + stringid.GenerateRandomID()
		file3 := "/" + stringid.GenerateRandomID()

		// Create container image with the files
		containerfile := fmt.Sprintf(`
FROM  %s
RUN touch %s
RUN touch %s
RUN touch %s`, ALPINE, file1, file2, file3)

		image := "podman-diff-test"
		podmanTest.BuildImage(containerfile, image, "true")

		// build a second image which used as base to compare against
		// using ALPINE does not work in CI, most likely due the extra vfs.imagestore
		containerfile = fmt.Sprintf(`
FROM  %s
RUN echo test
`, ALPINE)
		baseImage := "base-image"
		podmanTest.BuildImage(containerfile, baseImage, "true")

		session := podmanTest.Podman([]string{"image", "diff", image})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(1))
		Expect(session.OutputToString()).To(Equal("A " + file3))

		session = podmanTest.Podman([]string{"image", "diff", image, baseImage})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(4))
		Expect(session.OutputToString()).To(ContainSubstring("A " + file1))
		Expect(session.OutputToString()).To(ContainSubstring("A " + file2))
		Expect(session.OutputToString()).To(ContainSubstring("A " + file3))
	})

	It("podman image diff of single image", func() {
		session := podmanTest.Podman([]string{"image", "diff", BB})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())
	})

	It("podman image diff bogus image", func() {
		session := podmanTest.Podman([]string{"image", "diff", "1234", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman image diff of the same image", func() {
		session := podmanTest.Podman([]string{"image", "diff", ALPINE, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())
	})

	It("podman diff container and image with same name", func() {
		imagefile := "/" + stringid.GenerateRandomID()
		confile := "/" + stringid.GenerateRandomID()

		// Create container image with the files
		containerfile := fmt.Sprintf(`
FROM  %s
RUN touch %s`, ALPINE, imagefile)

		name := "podman-diff-test"
		podmanTest.BuildImage(containerfile, name, "false")

		session := podmanTest.Podman([]string{"run", "--name", name, ALPINE, "touch", confile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// podman diff prefers image over container when they have the same name
		session = podmanTest.Podman([]string{"diff", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring(imagefile))

		session = podmanTest.Podman([]string{"image", "diff", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring(imagefile))

		// container diff has to show the container
		session = podmanTest.Podman([]string{"container", "diff", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring(confile))
	})

})
