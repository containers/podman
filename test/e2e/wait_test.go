package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman wait", func() {
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

	It("podman wait on bogus container", func() {
		session := podmanTest.Podman([]string{"wait", "1234"})
		session.Wait()
		Expect(session.ExitCode()).To(Equal(125))

	})

	It("podman wait on a stopped container", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "ls"})
		session.Wait(10)
		cid := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"wait", cid})
		session.Wait()
	})

	It("podman wait on a sleeping container", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "10"})
		session.Wait(20)
		cid := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"wait", cid})
		session.Wait(20)
	})

	It("podman wait on latest container", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "10"})
		session.Wait(20)
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"wait", "-l"})
		session.Wait(20)
	})
})
