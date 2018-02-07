package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run exit", func() {
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

	It("podman run exit 125", func() {
		result := podmanTest.Podman([]string{"run", "--foobar", ALPINE, "ls", "$tmp"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
	})

	It("podman run exit 126", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "foobar"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(126))
	})

	It("podman run exit 127", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "/etc"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(127))
	})

	It("podman run exit 0", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman run exit 50", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		result := podmanTest.Podman([]string{"run", "registry.fedoraproject.org/fedora-minimal", "bash", "-c", "exit 50"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(50))
	})
})
