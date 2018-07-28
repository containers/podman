package integration

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run ns", func() {
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
		podmanTest.RestoreArtifact(fedoraMinimal)
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman run pidns test", func() {
		session := podmanTest.Podman([]string{"run", fedoraMinimal, "bash", "-c", "echo $$"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("1"))

		session = podmanTest.Podman([]string{"run", "--pid=host", fedoraMinimal, "bash", "-c", "echo $$"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Not(Equal("1")))

		session = podmanTest.Podman([]string{"run", "--pid=badpid", fedoraMinimal, "bash", "-c", "echo $$"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman run ipcns test", func() {
		testFile := "/dev/shm/podmantest"
		setup := podmanTest.SystemExec("touch", []string{testFile})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "--ipc=host", fedoraMinimal, "ls", testFile})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring(testFile))

		err := os.Remove(testFile)
		Expect(err).To(BeNil())
	})

	It("podman run bad ipc pid test", func() {
		session := podmanTest.Podman([]string{"run", "--ipc=badpid", fedoraMinimal, "bash", "-c", "echo $$"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).ToNot(Equal(0))
	})
})
