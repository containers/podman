package integration

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/onsi/gomega/gexec"
	"golang.org/x/sys/unix"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

const sigCatch = "for NUM in `seq 1 64`; do trap \"echo Received $NUM\" $NUM; done; echo READY; while :; do sleep 0.1; done"

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
		podmanTest.RestoreAllArtifacts()
		podmanTest.RestoreArtifact(fedoraMinimal)
	})

	AfterEach(func() {
		podmanTest.Cleanup()

	})

	Specify("signals are forwarded to container using sig-proxy", func() {
		signal := syscall.SIGPOLL
		session, pid := podmanTest.PodmanPID([]string{"run", "-it", "--name", "test1", fedoraMinimal, "bash", "-c", sigCatch})
		for i := 0; i < 10; i++ {
			foo := podmanTest.Podman([]string{"ps", "-q", "--filter", "name=test1"})
			foo.WaitWithDefaultTimeout()
			if len(foo.OutputToStringArray()) == 1 && foo.OutputToString() != "" {
				break
			}
			time.Sleep(1 * time.Second)
		}

		// Kill with given signal and do it 5 times because this stuff is racey!
		for j := 0; j < 5; j++ {
			if err := unix.Kill(pid, signal); err != nil {
				Fail(fmt.Sprintf("error killing podman process %d: %v", pid, err))
			}
			time.Sleep(5)
		}
		// Kill with -9 to guarantee the container dies
		killSession := podmanTest.Podman([]string{"kill", "-s", "9", "test1"})
		killSession.WaitWithDefaultTimeout()
		Expect(killSession.ExitCode()).To(Equal(0))

		session.WaitWithDefaultTimeout()
		ok, _ := session.GrepString(fmt.Sprintf("Received %d", signal))
		Expect(ok).To(BeTrue())
	})
	/*
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
			Expect(session.ExitCode()).To(Equal(0))
			ok, _ = session.GrepString(fmt.Sprintf("Received %d", signal))
			Expect(ok).To(BeFalse())
		})
	*/
})
