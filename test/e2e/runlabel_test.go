package integration

import (
	"fmt"
	"os"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var PodmanDockerfile = fmt.Sprintf(`
FROM  %s
LABEL RUN podman --version`, ALPINE)

var LsDockerfile = fmt.Sprintf(`
FROM  %s
LABEL RUN ls -la`, ALPINE)

var GlobalDockerfile = fmt.Sprintf(`
FROM %s
LABEL RUN echo \$GLOBAL_OPTS`, ALPINE)

var _ = Describe("podman container runlabel", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRemote("runlabel is not supported for remote connections")
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
	It("podman container runlabel --display", func() {
		image := "podman-runlabel-test:ls"
		podmanTest.BuildImage(LsDockerfile, image, "false")

		result := podmanTest.Podman([]string{"container", "runlabel", "--display", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(ContainSubstring(podmanTest.PodmanBinary + " -la"))

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})
	It("podman container runlabel bogus label should result in non-zero exit code", func() {
		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", ALPINE})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
		// should not panic when label missing the value or don't have the label
		Expect(result.LineInOutputContains("panic")).NotTo(BeTrue())
	})
	It("podman container runlabel bogus label in remote image should result in non-zero exit", func() {
		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", "docker.io/library/ubuntu:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
		// should not panic when label missing the value or don't have the label
		Expect(result.LineInOutputContains("panic")).NotTo(BeTrue())
	})

	It("podman container runlabel global options", func() {
		Skip("Test nonfunctional for podman-in-podman testing")
		image := "podman-global-test:ls"
		podmanTest.BuildImage(GlobalDockerfile, image, "false")
		result := podmanTest.Podman([]string{"--syslog", "--log-level", "debug", "container", "runlabel", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		Expect(result.OutputToString()).To(ContainSubstring("--syslog true"))
		Expect(result.OutputToString()).To(ContainSubstring("--log-level debug"))
		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("runlabel should fail with nonexistent authfile", func() {
		image := "podman-runlabel-test:podman"
		podmanTest.BuildImage(PodmanDockerfile, image, "false")

		// runlabel should fail with nonexistent authfile
		result := podmanTest.Podman([]string{"container", "runlabel", "--authfile", "/tmp/nonexistent", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Not(Equal(0)))

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})
})
