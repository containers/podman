package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman restart", func() {
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

	It("Podman restart bogus container", func() {
		session := podmanTest.Podman([]string{"start", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("Podman restart stopped container by name", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))

		session := podmanTest.Podman([]string{"restart", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("Podman restart stopped container by ID", func() {
		session := podmanTest.Podman([]string{"create", "-d", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		startSession := podmanTest.Podman([]string{"start", cid})
		startSession.WaitWithDefaultTimeout()
		Expect(startSession.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"restart", "test1"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))
	})

	It("Podman restart running container", func() {
		_ = podmanTest.RunTopContainer("test1")
		ok := WaitForContainer(&podmanTest)
		Expect(ok).To(BeTrue())

		session := podmanTest.Podman([]string{"restart", "--latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("Podman restart multiple containers", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))

		_, exitCode, _ = podmanTest.RunLsContainer("test2")
		Expect(exitCode).To(Equal(0))

		session := podmanTest.Podman([]string{"restart", "test1", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
})
