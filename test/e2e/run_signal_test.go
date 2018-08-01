package integration

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"golang.org/x/sys/unix"
)

// PodmanPID execs podman and returns its PID
func (p *PodmanTest) PodmanPID(args []string) (*PodmanSession, int) {
	podmanOptions := p.MakeOptions()
	podmanOptions = append(podmanOptions, strings.Split(p.StorageOptions, " ")...)
	podmanOptions = append(podmanOptions, args...)
	fmt.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	command := exec.Command(p.PodmanBinary, podmanOptions...)
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run podman command: %s", strings.Join(podmanOptions, " ")))
	}
	return &PodmanSession{session}, command.Process.Pid
}

const sigCatch = "trap \"echo FOO >> /h/fifo \" 8; echo READY >> /h/fifo; while :; do sleep 0.25; done"

var _ = Describe("Podman run with --sig-proxy", func() {
	var (
		tmpdir     string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tmpdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tmpdir)
		podmanTest.RestoreArtifact(fedoraMinimal)
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	Specify("signals are forwarded to container using sig-proxy", func() {
		signal := syscall.SIGFPE
		// Set up a socket for communication
		udsDir := filepath.Join(tmpdir, "socket")
		os.Mkdir(udsDir, 0700)
		udsPath := filepath.Join(udsDir, "fifo")
		syscall.Mkfifo(udsPath, 0600)

		_, pid := podmanTest.PodmanPID([]string{"run", "-it", "-v", fmt.Sprintf("%s:/h:Z", udsDir), fedoraMinimal, "bash", "-c", sigCatch})

		uds, _ := os.OpenFile(udsPath, os.O_RDONLY, 0600)
		defer uds.Close()

		// Wait for the script in the container to alert us that it is READY
		counter := 0
		for {
			buf := make([]byte, 1024)
			n, err := uds.Read(buf[:])
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
				os.Exit(1)
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
			n, err := uds.Read(buf[:])
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
				os.Exit(1)
			}
			counter++
		}
	})

	Specify("signals are not forwarded to container with sig-proxy false", func() {
		signal := syscall.SIGPOLL
		session, pid := podmanTest.PodmanPID([]string{"run", "--name", "test2", "--sig-proxy=false", fedoraMinimal, "bash", "-c", sigCatch})

		ok := WaitForContainer(&podmanTest)
		Expect(ok).To(BeTrue())

		// Kill with given signal
		// Should be no output, SIGPOLL is usually ignored
		if err := unix.Kill(pid, signal); err != nil {
			Fail(fmt.Sprintf("error killing podman process %d: %v", pid, err))
		}

		// Kill with -9 to guarantee the container dies
		killSession := podmanTest.Podman([]string{"kill", "-s", "9", "test2"})
		killSession.WaitWithDefaultTimeout()
		Expect(killSession.ExitCode()).To(Equal(0))

		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(137))
		ok, _ = session.GrepString(fmt.Sprintf("Received %d", signal))
		Expect(ok).To(BeFalse())
	})

})
