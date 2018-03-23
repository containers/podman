package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run", func() {
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

	It("podman run a container based on local image", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run a container based on local image with short options", func() {
		session := podmanTest.Podman([]string{"run", "-dt", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run a container based on remote image", func() {
		session := podmanTest.Podman([]string{"run", "-dt", BB_GLIBC, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run selinux grep test", func() {
		selinux := podmanTest.SystemExec("ls", []string{"/usr/sbin/selinuxenabled"})
		if selinux.ExitCode() != 0 {
			Skip("SELinux not enabled")
		}
		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", "label=level:s0:c1,c2", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("s0:c1,c2")
		Expect(match).Should(BeTrue())
	})

	It("podman run capabilities test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--cap-add", "all", ALPINE, "cat", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--rm", "--cap-add", "sys_admin", ALPINE, "cat", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--rm", "--cap-drop", "all", ALPINE, "cat", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--rm", "--cap-drop", "setuid", ALPINE, "cat", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run environment test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--env", "FOO=BAR", ALPINE, "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("BAR")
		Expect(match).Should(BeTrue())

		session = podmanTest.Podman([]string{"run", "--rm", "--env", "PATH=/bin", ALPINE, "printenv", "PATH"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ = session.GrepString("/bin")
		Expect(match).Should(BeTrue())

		os.Setenv("FOO", "BAR")
		session = podmanTest.Podman([]string{"run", "--rm", "--env", "FOO", ALPINE, "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ = session.GrepString("BAR")
		Expect(match).Should(BeTrue())
		os.Unsetenv("FOO")

		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "printenv"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// This currently does not work
		// Re-enable when hostname is an env variable
		//session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "sh", "-c", "printenv"})
		//session.Wait(10)
		//Expect(session.ExitCode()).To(Equal(0))
		//match, _ = session.GrepString("HOSTNAME")
		//Expect(match).Should(BeTrue())
	})

	It("podman run limits test", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		session := podmanTest.Podman([]string{"run", "--rm", "--ulimit", "rtprio=99", "--cap-add=sys_nice", fedoraMinimal, "cat", "/proc/self/sched"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--rm", "--ulimit", "nofile=2048:2048", fedoraMinimal, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("2048"))

		session = podmanTest.Podman([]string{"run", "--rm", "--ulimit", "nofile=1024:1028", fedoraMinimal, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("1024"))

		session = podmanTest.Podman([]string{"run", "--rm", "--oom-kill-disable=true", fedoraMinimal, "echo", "memory-hog"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--rm", "--oom-score-adj=100", fedoraMinimal, "cat", "/proc/self/oom_score_adj"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("100"))
	})

	It("podman run with volume flag", func() {
		Skip("Skip until we diagnose the regression of volume mounts")
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session := podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/run/test", mountPath), ALPINE, "cat", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/run/test rw,relatime"))

		mountPath = filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/run/test:ro", mountPath), ALPINE, "cat", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/run/test ro,relatime"))

		mountPath = filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/run/test:shared", mountPath), ALPINE, "cat", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/run/test rw,relatime, shared"))
	})

	It("podman run with cidfile", func() {
		session := podmanTest.Podman([]string{"run", "--cidfile", "/tmp/cidfile", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		err := os.Remove("/tmp/cidfile")
		Expect(err).To(BeNil())
	})

	It("podman run sysctl test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--sysctl", "net.core.somaxconn=65535", ALPINE, "sysctl", "net.core.somaxconn"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("net.core.somaxconn = 65535"))
	})

	It("podman run blkio-weight test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--blkio-weight=15", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.weight"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("15"))
	})

	It("podman run notify_socket", func() {
		sock := "/run/sock"
		os.Setenv("NOTIFY_SOCKET", sock)
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "printenv", "NOTIFY_SOCKET"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString(sock)
		Expect(match).Should(BeTrue())
		os.Unsetenv("NOTIFY_SOCKET")
	})

	It("podman run log-opt", func() {
		log := filepath.Join(podmanTest.TempDir, "/container.log")
		session := podmanTest.Podman([]string{"run", "--rm", "--log-opt", fmt.Sprintf("path=%s", log), ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		fmt.Println(session.OutputToString())
		Expect(session.ExitCode()).To(Equal(0))
		_, err := os.Stat(log)
		Expect(err).To(BeNil())
		_ = os.Remove(log)
	})

	It("podman run tagged image", func() {
		tag := podmanTest.Podman([]string{"tag", "busybox", "bb"})
		tag.WaitWithDefaultTimeout()
		Expect(tag.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "--rm", "bb", "ls"})
		session.WaitWithDefaultTimeout()
		fmt.Println(session.OutputToString())
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run with secrets", func() {
		containersDir := "/usr/share/containers"
		err := os.MkdirAll(containersDir, 0755)
		Expect(err).To(BeNil())

		secretsDir := filepath.Join(podmanTest.TempDir, "rhel", "secrets")
		err = os.MkdirAll(secretsDir, 0755)
		Expect(err).To(BeNil())

		mountsFile := filepath.Join(containersDir, "mounts.conf")
		mountString := secretsDir + ":/run/secrets"
		err = ioutil.WriteFile(mountsFile, []byte(mountString), 0755)
		Expect(err).To(BeNil())

		secretsFile := filepath.Join(secretsDir, "test.txt")
		secretsString := "Testing secrets mount. I am mounted!"
		err = ioutil.WriteFile(secretsFile, []byte(secretsString), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "cat", "/run/secrets/test.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Equal(secretsString))

		err = os.RemoveAll(containersDir)
		Expect(err).To(BeNil())
	})

})
