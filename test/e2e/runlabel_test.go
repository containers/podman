package integration

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var PodmanDockerfile = `
FROM  alpine:latest
LABEL RUN podman --version`

var LsDockerfile = `
FROM  alpine:latest
LABEL RUN ls -la`

var _ = Describe("podman container runlabel", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))

	})

	It("podman container runlabel (podman --version)", func() {
		image := "podman-runlabel-test:podman"
		podmanTest.BuildImage(PodmanDockerfile, image, "false")

		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman container runlabel (ls -la)", func() {
		image := "podman-runlabel-test:ls"
		podmanTest.BuildImage(LsDockerfile, image, "false")

		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})
})
