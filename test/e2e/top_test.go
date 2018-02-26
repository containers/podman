package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman top", func() {
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

	It("podman top without container name or id", func() {
		result := podmanTest.Podman([]string{"top"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
	})

	It("podman top on bogus container", func() {
		result := podmanTest.Podman([]string{"top", "1234"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
	})

	It("podman top on non-running container", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"top", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
	})

	It("podman top on container", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"top", "-l"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman top with options", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"top", session.OutputToString(), "-o", "pid,fuser,f,comm,label"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman top on container invalid options", func() {
		sleep := podmanTest.RunSleepContainer("")
		sleep.WaitWithDefaultTimeout()
		Expect(sleep.ExitCode()).To(Equal(0))
		cid := sleep.OutputToString()

		result := podmanTest.Podman([]string{"top", cid, "-o time"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
	})

})
