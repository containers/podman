package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman rmi", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
		image1     = "docker.io/library/alpine:latest"
		image3     = "docker.io/library/busybox:glibc"
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

	})

	It("podman rmi bogus image", func() {
		session := podmanTest.Podman([]string{"rmi", "debian:6.0.10"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))

	})

	It("podman rmi with fq name", func() {
		session := podmanTest.Podman([]string{"rmi", image1})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

	})

	It("podman rmi with short name", func() {
		session := podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

	})

	It("podman rmi all images", func() {
		podmanTest.PullImages([]string{image3})
		session := podmanTest.Podman([]string{"rmi", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

	})

	It("podman rmi all images forceably with short options", func() {
		podmanTest.PullImages([]string{image3})
		session := podmanTest.Podman([]string{"rmi", "-fa"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

	})

	It("podman rmi tagged image", func() {
		setup := podmanTest.Podman([]string{"images", "-q", ALPINE})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"tag", "alpine", "foo:bar", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"images", "-q", "foo"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		Expect(result.LineInOutputContains(setup.OutputToString())).To(BeTrue())
	})

	It("podman rmi image with tags by ID cannot be done without force", func() {
		setup := podmanTest.Podman([]string{"images", "-q", ALPINE})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		alpineId := setup.OutputToString()

		session := podmanTest.Podman([]string{"tag", "alpine", "foo:bar", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Trying without --force should fail
		result := podmanTest.Podman([]string{"rmi", alpineId})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))

		// With --force it should work
		resultForce := podmanTest.Podman([]string{"rmi", "-f", alpineId})
		resultForce.WaitWithDefaultTimeout()
		Expect(resultForce.ExitCode()).To(Equal(0))
	})
})
