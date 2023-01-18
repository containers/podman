package integration

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"golang.org/x/sys/unix"
)

const sigCatch = "trap \"echo FOO >> /h/fifo \" 8; echo READY >> /h/fifo; while :; do sleep 0.25; done"
const sigCatch2 = "trap \"echo Received\" SIGFPE; while :; do sleep 0.25; done"

var _ = Describe("Podman run with --sig-proxy", func() {
	var (
		tmpdir     string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tmpdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tmpdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	Specify("signals are forwarded to container using sig-proxy", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("Doesn't work on ppc64le")
		}
		signal := syscall.SIGFPE
		// Set up a socket for communication
		udsDir := filepath.Join(tmpdir, "socket")
		err := os.Mkdir(udsDir, 0700)
		Expect(err).ToNot(HaveOccurred())
		udsPath := filepath.Join(udsDir, "fifo")
		err = syscall.Mkfifo(udsPath, 0600)
		Expect(err).ToNot(HaveOccurred())
		if isRootless() {
			err = podmanTest.RestoreArtifact(fedoraMinimal)
			Expect(err).ToNot(HaveOccurred())
		}
		_, pid := podmanTest.PodmanPID([]string{"run", "-it", "-v", fmt.Sprintf("%s:/h:Z", udsDir), fedoraMinimal, "bash", "-c", sigCatch})

		uds, _ := os.OpenFile(udsPath, os.O_RDONLY|syscall.O_NONBLOCK, 0600)
		defer uds.Close()

		// Wait for the script in the container to alert us that it is READY
		counter := 0
		for {
			buf := make([]byte, 1024)
			n, err := uds.Read(buf)
			if err != nil && err != io.EOF {
				fmt.Println(err)
				return
			}
			data := string(buf[0:n])
			if strings.Contains(data, "READY") {
				break
			}
			time.Sleep(1 * time.Second)
			if counter == 15 {
				Fail("Timed out waiting for READY from container")
			}
			counter++
		}
		// Ok, container is up and running now and so is the script

		if err := unix.Kill(pid, signal); err != nil {
			Fail(fmt.Sprintf("error killing podman process %d: %v", pid, err))
		}

		// The sending of the signal above will send FOO to the socket; here we
		// listen to the socket for that.
		counter = 0
		for {
			buf := make([]byte, 1024)
			n, err := uds.Read(buf)
			if err != nil {
				fmt.Println(err)
				return
			}
			data := string(buf[0:n])
			if strings.Contains(data, "FOO") {
				break
			}
			time.Sleep(1 * time.Second)
			if counter == 15 {
				Fail("timed out waiting for FOO from container")
			}
			counter++
		}
	})

	Specify("signals are not forwarded to container with sig-proxy false", func() {
		signal := syscall.SIGFPE
		if isRootless() {
			err = podmanTest.RestoreArtifact(fedoraMinimal)
			Expect(err).ToNot(HaveOccurred())
		}
		session, pid := podmanTest.PodmanPID([]string{"run", "--name", "test2", "--sig-proxy=false", fedoraMinimal, "bash", "-c", sigCatch2})

		Expect(WaitForContainer(podmanTest)).To(BeTrue(), "WaitForContainer()")

		// Kill with given signal
		// Should be no output, SIGPOLL is usually ignored
		if err := unix.Kill(pid, signal); err != nil {
			Fail(fmt.Sprintf("error killing podman process %d: %v", pid, err))
		}

		// Kill with -9 to guarantee the container dies
		killSession := podmanTest.Podman([]string{"kill", "-s", "9", "test2"})
		killSession.WaitWithDefaultTimeout()
		Expect(killSession).Should(Exit(0))

		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.OutputToString()).To(Not(ContainSubstring("Received")))
	})

})
