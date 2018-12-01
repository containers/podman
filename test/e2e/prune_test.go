package integration

import (
	"fmt"
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

var _ = Describe("Podman rm", func() {
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
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
	})

	It("podman image prune unused images", func() {
		prune := podmanTest.Podman([]string{"image", "prune"})
		prune.WaitWithDefaultTimeout()
		Expect(prune.ExitCode()).To(Equal(0))

		images := podmanTest.Podman([]string{"images", "-a"})
		images.WaitWithDefaultTimeout()
		// all images are unused, so they all should be deleted!
		Expect(len(images.OutputToStringArray())).To(Equal(0))
	})

})
