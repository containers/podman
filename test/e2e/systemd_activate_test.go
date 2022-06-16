package integration

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	testUtils "github.com/containers/podman/v4/test/utils"
	podmanUtils "github.com/containers/podman/v4/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Systemd activate", func() {
	var tempDir string
	var err error
	var podmanTest *PodmanTestIntegration
	var activate string

	BeforeEach(func() {
		tempDir, err = testUtils.CreateTempDirInTempDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		podmanTest = PodmanTestCreate(tempDir)
		podmanTest.Setup()

		SkipIfRemote("Testing stopped service requires both podman and podman-remote binaries")

		activate, err = exec.LookPath("systemd-socket-activate")
		if err != nil {
			activate = "/usr/bin/systemd-socket-activate"
		}
		stat, err := os.Stat(activate)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			Skip(activate + " required for systemd activation tests")
		case stat.Mode()&0111 == 0:
			Skip("Unable to execute " + activate)
		case err != nil:
			Skip(err.Error())
		}
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		processTestResult(CurrentGinkgoTestDescription())
	})

	It("stop podman.service", func() {
		// systemd-socket-activate does not support DNS lookups
		host := "127.0.0.1"
		port, err := podmanUtils.GetRandomPort()
		Expect(err).ToNot(HaveOccurred())
		addr := net.JoinHostPort(host, strconv.Itoa(port))

		activateSession := testUtils.StartSystemExec(activate, []string{
			"--listen", addr,
			podmanTest.PodmanBinary,
			"--root=" + filepath.Join(tempDir, "server_root"),
			"system", "service",
			"--time=0",
		})
		Expect(activateSession.Exited).ShouldNot(Receive(), "Failed to start podman service")

		// Curried functions for specialized podman calls
		podmanRemote := func(args ...string) *testUtils.PodmanSession {
			args = append([]string{"--url", "tcp://" + addr}, args...)
			return testUtils.SystemExec(podmanTest.RemotePodmanBinary, args)
		}

		podman := func(args ...string) *testUtils.PodmanSession {
			args = append([]string{"--root", filepath.Join(tempDir, "server_root")}, args...)
			return testUtils.SystemExec(podmanTest.PodmanBinary, args)
		}

		containerName := "top_" + testUtils.RandomString(8)
		apiSession := podmanRemote(
			"create", "--tty", "--name", containerName, "--entrypoint", "top",
			"quay.io/libpod/alpine_labels:latest",
		)
		Expect(apiSession).Should(Exit(0))

		apiSession = podmanRemote("start", containerName)
		Expect(apiSession).Should(Exit(0))

		apiSession = podmanRemote("inspect", "--format={{.State.Running}}", containerName)
		Expect(apiSession).Should(Exit(0))
		Expect(apiSession.OutputToString()).To(Equal("true"))

		// Emulate 'systemd stop podman.service'
		activateSession.Signal(syscall.SIGTERM)
		time.Sleep(100 * time.Millisecond)
		Eventually(activateSession).Should(Exit(0))

		abiSession := podman("inspect", "--format={{.State.Running}}", containerName)
		Expect(abiSession).To(Exit(0))
		Expect(abiSession.OutputToString()).To(Equal("true"))
	})

	It("invalid systemd file descriptor", func() {
		host := "127.0.0.1"
		port, err := podmanUtils.GetRandomPort()
		Expect(err).ToNot(HaveOccurred())

		addr := net.JoinHostPort(host, strconv.Itoa(port))

		// start systemd activation with datagram socket
		activateSession := testUtils.StartSystemExec(activate, []string{
			"--datagram", "--listen", addr,
			podmanTest.PodmanBinary,
			"--root=" + filepath.Join(tempDir, "server_root"),
			"system", "service",
			"--time=0",
		})
		Expect(activateSession.Exited).ShouldNot(Receive(), "Failed to start podman service")

		// we have to wait for systemd-socket-activate to become ready
		time.Sleep(1 * time.Second)

		// now dial the socket to start podman
		conn, err := net.Dial("udp", addr)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		_, err = conn.Write([]byte("test"))
		Expect(err).ToNot(HaveOccurred())

		// wait for podman to exit
		activateSession.Wait(10)
		Expect(activateSession).To(Exit(125))
		Expect(activateSession.ErrorToString()).To(ContainSubstring("Error: unexpected fd received from systemd: cannot listen on it"))
	})
})
