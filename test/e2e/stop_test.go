package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman stop", func() {
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

	It("podman stop bogus container", func() {
		session := podmanTest.Podman([]string{"stop", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman stop container by id", func() {
		session := podmanTest.RunSleepContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"stop", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman stop container by name", func() {
		session := podmanTest.RunSleepContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"stop", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman stop all containers", func() {
		session := podmanTest.RunSleepContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid1 := session.OutputToString()

		session = podmanTest.RunSleepContainer("test2")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid2 := session.OutputToString()

		session = podmanTest.RunSleepContainer("test3")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid3 := session.OutputToString()

		session = podmanTest.Podman([]string{"stop", "-a", "-t", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		output := session.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(output).To(ContainSubstring(cid3))
	})

	It("podman stop latest containers", func() {
		session := podmanTest.RunSleepContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"stop", "-l", "-t", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman stop --time compatibilty", func() {
		session := podmanTest.RunSleepContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"stop", "-l", "--time", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
})
