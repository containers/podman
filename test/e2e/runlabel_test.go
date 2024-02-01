package integration

import (
	"fmt"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var PodmanDockerfile = fmt.Sprintf(`
FROM  %s
LABEL RUN podman --version`, ALPINE)

var LsDockerfile = fmt.Sprintf(`
FROM  %s
LABEL RUN ls -la`, ALPINE)

var PodmanRunlabelNameDockerfile = fmt.Sprintf(`
FROM  %s
LABEL RUN podman run --name NAME IMAGE`, ALPINE)

var _ = Describe("podman container runlabel", func() {

	BeforeEach(func() {
		SkipIfRemote("runlabel is not supported for remote connections")
	})

	It("podman container runlabel (podman --version)", func() {
		image := "podman-runlabel-test:podman"
		podmanTest.BuildImage(PodmanDockerfile, image, "false")

		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman container runlabel (ls -la)", func() {
		image := "podman-runlabel-test:ls"
		podmanTest.BuildImage(LsDockerfile, image, "false")

		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})
	It("podman container runlabel --display", func() {
		image := "podman-runlabel-test:ls"
		podmanTest.BuildImage(LsDockerfile, image, "false")

		result := podmanTest.Podman([]string{"container", "runlabel", "--display", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(podmanTest.PodmanBinary + " -la"))

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})
	It("podman container runlabel bogus label should result in non-zero exit code", func() {
		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", ALPINE})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
		// should not panic when label missing the value or don't have the label
		Expect(result.OutputToString()).To(Not(ContainSubstring("panic")))
	})
	It("podman container runlabel bogus label in remote image should result in non-zero exit", func() {
		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", "docker.io/library/ubuntu:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
		// should not panic when label missing the value or don't have the label
		Expect(result.OutputToString()).To(Not(ContainSubstring("panic")))
	})

	It("runlabel should fail with nonexistent authfile", func() {
		image := "podman-runlabel-test:podman"
		podmanTest.BuildImage(PodmanDockerfile, image, "false")

		// runlabel should fail with nonexistent authfile
		result := podmanTest.Podman([]string{"container", "runlabel", "--authfile", "/tmp/nonexistent", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman container runlabel name removes tag from image", func() {
		image := "podman-runlabel-name:sometag"
		podmanTest.BuildImage(PodmanRunlabelNameDockerfile, image, "false")

		result := podmanTest.Podman([]string{"container", "runlabel", "--display", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal("command: " + podmanTest.PodmanBinary + " run --name podman-runlabel-name localhost/" + image))

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})
})
