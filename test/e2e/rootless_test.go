package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func canExec() bool {
	const nsGetParent = 0xb702

	u, err := os.Open("/proc/self/ns/user")
	if err != nil {
		return false
	}
	defer u.Close()

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, u.Fd(), uintptr(nsGetParent), 0)
	return errno != syscall.ENOTTY
}

var _ = Describe("Podman rootless", func() {
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
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman rootless help|version", func() {
		commands := []string{"help", "version"}
		for _, v := range commands {
			env := os.Environ()
			cmd := podmanTest.PodmanAsUser([]string{v}, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))
		}
	})

	chownFunc := func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(p, 1000, 1000)
	}

	type rootlessCB func(test PodmanTest, xdgRuntimeDir string, home string, mountPath string)

	runInRootlessContext := func(cb rootlessCB) {
		// Check if we can create an user namespace
		err := exec.Command("unshare", "-r", "echo", "hello").Run()
		if err != nil {
			Skip("User namespaces not supported.")
		}
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid := setup.OutputToString()

		mount := podmanTest.Podman([]string{"mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount.ExitCode()).To(Equal(0))
		mountPath := mount.OutputToString()

		err = filepath.Walk(tempdir, chownFunc)
		Expect(err).To(BeNil())

		tempdir, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())
		rootlessTest := PodmanCreate(tempdir)
		err = filepath.Walk(tempdir, chownFunc)
		Expect(err).To(BeNil())

		xdgRuntimeDir, err := ioutil.TempDir("/run", "")
		Expect(err).To(BeNil())
		defer os.RemoveAll(xdgRuntimeDir)
		err = filepath.Walk(xdgRuntimeDir, chownFunc)
		Expect(err).To(BeNil())

		home, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())
		err = filepath.Walk(home, chownFunc)
		Expect(err).To(BeNil())

		cb(rootlessTest, xdgRuntimeDir, home, mountPath)

		umount := podmanTest.Podman([]string{"umount", cid})
		umount.WaitWithDefaultTimeout()
		Expect(umount.ExitCode()).To(Equal(0))
	}

	It("podman rootless pod", func() {
		f := func(rootlessTest PodmanTest, xdgRuntimeDir string, home string, mountPath string) {
			env := os.Environ()
			env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir))
			env = append(env, fmt.Sprintf("HOME=%s", home))
			env = append(env, "PODMAN_ALLOW_SINGLE_ID_MAPPING_IN_USERNS=1")

			cmd := rootlessTest.PodmanAsUser([]string{"pod", "create", "--infra=false"}, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))
			podId := cmd.OutputToString()

			args := []string{"run", "--pod", podId, "--rootfs", mountPath, "echo", "hello"}
			cmd = rootlessTest.PodmanAsUser(args, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))
			Expect(cmd.LineInOutputContains("hello")).To(BeTrue())
		}
		runInRootlessContext(f)
	})

	It("podman rootless search", func() {
		xdgRuntimeDir, err := ioutil.TempDir("/run", "")
		Expect(err).To(BeNil())
		defer os.RemoveAll(xdgRuntimeDir)
		err = filepath.Walk(xdgRuntimeDir, chownFunc)
		Expect(err).To(BeNil())

		home, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())
		err = filepath.Walk(home, chownFunc)
		Expect(err).To(BeNil())

		env := os.Environ()
		env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir))
		env = append(env, fmt.Sprintf("HOME=%s", home))
		cmd := podmanTest.PodmanAsUser([]string{"search", "docker.io/busybox"}, 1000, 1000, env)
		cmd.WaitWithDefaultTimeout()
		Expect(cmd.ExitCode()).To(Equal(0))
	})

	runRootlessHelper := func(args []string) {
		f := func(rootlessTest PodmanTest, xdgRuntimeDir string, home string, mountPath string) {
			env := os.Environ()
			env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir))
			env = append(env, fmt.Sprintf("HOME=%s", home))
			env = append(env, "PODMAN_ALLOW_SINGLE_ID_MAPPING_IN_USERNS=1")

			allArgs := append([]string{"run"}, args...)
			allArgs = append(allArgs, "--rootfs", mountPath, "echo", "hello")
			cmd := rootlessTest.PodmanAsUser(allArgs, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))
			Expect(cmd.LineInOutputContains("hello")).To(BeTrue())

			cmd = rootlessTest.PodmanAsUser([]string{"rm", "-l", "-f"}, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))

			allArgs = append([]string{"run", "-d"}, args...)
			allArgs = append(allArgs, "--security-opt", "seccomp=unconfined", "--rootfs", mountPath, "top")
			cmd = rootlessTest.PodmanAsUser(allArgs, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))

			canUseExec := canExec()

			if canUseExec {
				cmd = rootlessTest.PodmanAsUser([]string{"top", "-l"}, 1000, 1000, env)
				cmd.WaitWithDefaultTimeout()
				Expect(cmd.ExitCode()).To(Equal(0))
			}

			cmd = rootlessTest.PodmanAsUser([]string{"rm", "-l", "-f"}, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))

			allArgs = append([]string{"run", "-d"}, args...)
			allArgs = append(allArgs, "--security-opt", "seccomp=unconfined", "--rootfs", mountPath, "unshare", "-r", "unshare", "-r", "top")
			cmd = rootlessTest.PodmanAsUser(allArgs, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))

			cmd = rootlessTest.PodmanAsUser([]string{"kill", "-l"}, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))

			cmd = rootlessTest.PodmanAsUser([]string{"start", "-l"}, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))

			cmd = rootlessTest.PodmanAsUser([]string{"stop", "-l", "-t", "0"}, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))

			cmd = rootlessTest.PodmanAsUser([]string{"start", "-l"}, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))

			if !canUseExec {
				Skip("ioctl(NS_GET_PARENT) not supported.")
			}

			cmd = rootlessTest.PodmanAsUser([]string{"exec", "-l", "echo", "hello"}, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.ExitCode()).To(Equal(0))
			Expect(cmd.LineInOutputContains("hello")).To(BeTrue())
		}
		runInRootlessContext(f)
	}

	It("podman rootless rootfs", func() {
		runRootlessHelper([]string{})
	})

	It("podman rootless rootfs --net host", func() {
		runRootlessHelper([]string{"--net", "host"})
	})

	It("podman rootless rootfs --privileged", func() {
		runRootlessHelper([]string{"--privileged"})
	})

	It("podman rootless rootfs --net host --privileged", func() {
		runRootlessHelper([]string{"--net", "host", "--privileged"})
	})

	It("podman rootless rootfs --uts host", func() {
		runRootlessHelper([]string{"--uts", "host"})
	})

	It("podman rootless rootfs --ipc host", func() {
		runRootlessHelper([]string{"--ipc", "host"})
	})
})
