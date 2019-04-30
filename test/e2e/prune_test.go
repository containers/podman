package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var pruneImage = `
FROM  alpine:latest
LABEL RUN podman --version
RUN apk update
RUN apk add bash`

var _ = Describe("Podman prune", func() {
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman container prune containers", func() {
		top := podmanTest.RunTopContainer("")
		top.WaitWithDefaultTimeout()
		Expect(top.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"run", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		prune := podmanTest.Podman([]string{"container", "prune"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman image prune none images", func() {
		SkipIfRemote()
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")

		none := podmanTest.Podman([]string{"images", "-a"})
		none.WaitWithDefaultTimeout()
		Expect(none.ExitCode()).To(Equal(0))
		hasNone, _ := none.GrepString("<none>")
		Expect(hasNone).To(BeTrue())

		prune := podmanTest.Podman([]string{"image", "prune"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		after := podmanTest.Podman([]string{"images", "-a"})
		after.WaitWithDefaultTimeout()
		Expect(none.ExitCode()).To(Equal(0))
		hasNoneAfter, _ := after.GrepString("<none>")
		Expect(hasNoneAfter).To(BeFalse())
		Expect(len(after.OutputToStringArray()) > 1).To(BeTrue())
	})

	It("podman image prune unused images", func() {
		prune := podmanTest.Podman([]string{"image", "prune", "-a"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		images := podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		// all images are unused, so they all should be deleted!
		Expect(len(images.OutputToStringArray())).To(Equal(0))
	})

	It("podman system image prune unused images", func() {
		SkipIfRemote()
		podmanTest.BuildImage(pruneImage, "alpine_bash:latest", "true")
		prune := podmanTest.Podman([]string{"system", "prune", "-a", "--force"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		images := podmanTest.Podman([]string{"images", "-aq"})
		images.WaitWithDefaultTimeout()
		// all images are unused, so they all should be deleted!
		Expect(len(images.OutputToStringArray())).To(Equal(0))
	})

	It("podman system prune pods", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		pods := podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods.ExitCode()).To(Equal(0))
		Expect(len(pods.OutputToStringArray())).To(Equal(3))

		prune := podmanTest.Podman([]string{"system", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		pods = podmanTest.Podman([]string{"pod", "ps"})
		pods.WaitWithDefaultTimeout()
		Expect(pods.ExitCode()).To(Equal(0))
		Expect(len(pods.OutputToStringArray())).To(Equal(2))
	})
})
