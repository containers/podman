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

	It("podman images in JSON format", func() {
		session := podmanTest.Podman([]string{"images", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman images with short options", func() {
		session := podmanTest.Podman([]string{"images", "-qn"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 1))
	})
})
