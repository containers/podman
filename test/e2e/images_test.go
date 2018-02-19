package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman images", func() {
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

	})

	It("podman images", func() {
		session := podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 2))
		Expect(session.LineInOuputStartsWith("docker.io/library/alpine")).To(BeTrue())
		Expect(session.LineInOuputStartsWith("docker.io/library/busybox")).To(BeTrue())
	})

	It("podman images with digests", func() {
		session := podmanTest.Podman([]string{"images", "--digests"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 2))
		Expect(session.LineInOuputStartsWith("docker.io/library/alpine")).To(BeTrue())
		Expect(session.LineInOuputStartsWith("docker.io/library/busybox")).To(BeTrue())
	})

	It("podman images in JSON format", func() {
		session := podmanTest.Podman([]string{"images", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman images in GO template format", func() {
		session := podmanTest.Podman([]string{"images", "--format={{.ID}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman images with short options", func() {
		session := podmanTest.Podman([]string{"images", "-qn"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman images filter by image name", func() {
		session := podmanTest.Podman([]string{"images", "-q", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(1))
	})

	It("podman images filter before image", func() {
		dockerfile := `FROM docker.io/library/alpine:latest
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest")
		result := podmanTest.Podman([]string{"images", "-q", "-f", "before=foobar.com/before:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(Equal(2))
	})

	It("podman images filter after image", func() {
		rmi := podmanTest.Podman([]string{"rmi", "busybox"})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		dockerfile := `FROM docker.io/library/alpine:latest
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest")
		result := podmanTest.Podman([]string{"images", "-q", "-f", "after=docker.io/library/alpine:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(Equal(1))
	})

	It("podman images filter dangling", func() {
		dockerfile := `FROM docker.io/library/alpine:latest
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest")
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest")
		result := podmanTest.Podman([]string{"images", "-q", "-f", "dangling=true"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(Equal(1))
	})
})
