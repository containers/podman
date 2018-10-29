package integration

import (
	"fmt"
	"os"
	"strings"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run ns", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
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
		setup := SystemExec("ls", []string{"--inode", "-d", "/dev/shm"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		hostShm := setup.OutputToString()

		session := podmanTest.Podman([]string{"run", "--ipc=host", fedoraMinimal, "ls", "--inode", "-d", "/dev/shm"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal(hostShm))
	})

	It("podman run ipcns ipcmk host test", func() {
		setup := SystemExec("ipcmk", []string{"-M", "1024"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		output := strings.Split(setup.OutputToString(), " ")
		ipc := output[len(output)-1]
		session := podmanTest.Podman([]string{"run", "--ipc=host", fedoraMinimal, "ipcs", "-m", "-i", ipc})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		setup = SystemExec("ipcrm", []string{"-m", ipc})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
	})

	It("podman run ipcns ipcmk container test", func() {
		setup := podmanTest.Podman([]string{"run", "-d", "--name", "test1", fedoraMinimal, "sleep", "999"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "test1", "ipcmk", "-M", "1024"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		output := strings.Split(session.OutputToString(), " ")
		ipc := output[len(output)-1]
		session = podmanTest.Podman([]string{"run", "--ipc=container:test1", fedoraMinimal, "ipcs", "-m", "-i", ipc})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run bad ipc pid test", func() {
		session := podmanTest.Podman([]string{"run", "--ipc=badpid", fedoraMinimal, "bash", "-c", "echo $$"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).ToNot(Equal(0))
	})
})
