// +build !remoteclient

package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var PodmanDockerfile = `
FROM  alpine:latest
LABEL RUN podman --version`

var LsDockerfile = `
FROM  alpine:latest
LABEL RUN ls -la`

var GlobalDockerfile = `
FROM alpine:latest
LABEL RUN echo \$GLOBAL_OPTS
`

var _ = Describe("podman container runlabel", func() {
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
	It("podman container runlabel bogus label should result in non-zero exit code", func() {
		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", ALPINE})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
	})
	It("podman container runlabel bogus label in remote image should result in non-zero exit", func() {
		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", "docker.io/library/ubuntu:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())

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

	It("runlabel should fail with nonexist authfile", func() {
		SkipIfRemote()
		image := "podman-runlabel-test:podman"
		podmanTest.BuildImage(PodmanDockerfile, image, "false")

		// runlabel should fail with nonexist authfile
		result := podmanTest.Podman([]string{"container", "runlabel", "--authfile", "/tmp/nonexist", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Not(Equal(0)))

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})
})
