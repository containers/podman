package integration

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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

	BeforeEach(func() {
		tempDir, err = testUtils.CreateTempDirInTempDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		podmanTest = PodmanTestCreate(tempDir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		processTestResult(CurrentGinkgoTestDescription())
	})

	It("stop podman.service", func() {
		SkipIfRemote("Testing stopped service requires both podman and podman-remote binaries")

		activate, err := exec.LookPath("systemd-socket-activate")
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

		// systemd-socket-activate does not support DNS lookups
		host := "127.0.0.1"
		port, err := podmanUtils.GetRandomPort()
		Expect(err).ToNot(HaveOccurred())

		activateSession := testUtils.StartSystemExec(activate, []string{
			fmt.Sprintf("--listen=%s:%d", host, port),
			podmanTest.PodmanBinary,
			"--root=" + filepath.Join(tempDir, "server_root"),
			"system", "service",
			"--time=0",
		})
		Expect(activateSession.Exited).ShouldNot(Receive(), "Failed to start podman service")

		// Curried functions for specialized podman calls
		podmanRemote := func(args ...string) *testUtils.PodmanSession {
			args = append([]string{"--url", fmt.Sprintf("tcp://%s:%d", host, port)}, args...)
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
})
