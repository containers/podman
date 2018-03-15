package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run restart containers", func() {
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

	It("Podman start after successful run", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"start", "--attach", "--latest"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))
	})

	It("Podman start after signal kill", func() {
		session := podmanTest.RunTopContainer("test1")
		ok := WaitForContainer(&podmanTest)
		Expect(ok).To(BeTrue())

		killSession := podmanTest.Podman([]string{"kill", "-s", "9", "test1"})
		killSession.WaitWithDefaultTimeout()
		Expect(killSession.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"start", "--attach", "test1"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))
	})
})
