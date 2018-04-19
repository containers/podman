package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman search", func() {
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()

	})

	It("podman search", func() {
		search := podmanTest.Podman([]string{"search", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.LineInOutputContains("docker.io/library/alpine")).To(BeTrue())
	})

	It("podman search registry flag", func() {
		search := podmanTest.Podman([]string{"search", "--registry", "registry.fedoraproject.org", "fedora-minimal"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.LineInOutputContains("fedoraproject.org/fedora-minimal")).To(BeTrue())
	})

	It("podman search format flag", func() {
		search := podmanTest.Podman([]string{"search", "--format", "table {{.Index}} {{.Name}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.LineInOutputContains("docker.io/library/alpine")).To(BeTrue())
	})

	It("podman search no-trunc flag", func() {
		search := podmanTest.Podman([]string{"search", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.LineInOutputContains("docker.io/library/alpine")).To(BeTrue())
	})

	It("podman search limit flag", func() {
		search := podmanTest.Podman([]string{"search", "--limit", "3", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(Equal(4))
	})
})
