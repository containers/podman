package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/libpod/test/utils"
	"github.com/mrunalp/fileutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run", func() {
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman run a container based on local image", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run a container based on a complex local image name", func() {
		imageName := strings.TrimPrefix(nginx, "quay.io/")
		podmanTest.RestoreArtifact(nginx)
		session := podmanTest.Podman([]string{"run", imageName, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ErrorToString()).ToNot(ContainSubstring("Trying to pull"))
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run a container based on on a short name with localhost", func() {
		podmanTest.RestoreArtifact(nginx)
		tag := podmanTest.Podman([]string{"tag", nginx, "localhost/libpod/alpine_nginx:latest"})
		tag.WaitWithDefaultTimeout()

		rmi := podmanTest.Podman([]string{"rmi", nginx})
		rmi.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"run", "libpod/alpine_nginx:latest", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ErrorToString()).ToNot(ContainSubstring("Trying to pull"))
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run a container based on local image with short options", func() {
		session := podmanTest.Podman([]string{"run", "-dt", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run a container based on local image with short options and args", func() {
		// regression test for #714
		session := podmanTest.Podman([]string{"run", ALPINE, "find", "/etc", "-name", "hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("/etc/hosts")
		Expect(match).Should(BeTrue())
	})

	It("podman run a container based on remote image", func() {
		session := podmanTest.Podman([]string{"run", "-dt", BB_GLIBC, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run seccomp test", func() {
		jsonFile := filepath.Join(podmanTest.TempDir, "seccomp.json")
		in := []byte(`{"defaultAction":"SCMP_ACT_ALLOW","syscalls":[{"name":"getcwd","action":"SCMP_ACT_ERRNO"}]}`)
		err := WriteJsonFile(in, jsonFile)
		if err != nil {
			fmt.Println(err)
			Skip("Failed to prepare seccomp.json for test.")
		}

		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", strings.Join([]string{"seccomp=", jsonFile}, ""), ALPINE, "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
		match, _ := session.GrepString("Operation not permitted")
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
		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "sh", "-c", "printenv"})
		session.Wait(10)
		Expect(session.ExitCode()).To(Equal(0))
		match, _ = session.GrepString("HOSTNAME")
		Expect(match).Should(BeTrue())
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

	It("podman run with --mount flag", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("skip failing test on ppc64le")
		}
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session := podmanTest.Podman([]string{"run", "--rm", "--mount", fmt.Sprintf("type=bind,src=%s,target=/run/test", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/run/test rw"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", fmt.Sprintf("type=bind,src=%s,target=/run/test,ro", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/run/test ro"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", fmt.Sprintf("type=bind,src=%s,target=/run/test,shared", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		found, matches := session.GrepString("/run/test")
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))
		Expect(matches[0]).To(ContainSubstring("shared"))

		mountPath = filepath.Join(podmanTest.TempDir, "scratchpad")
		os.Mkdir(mountPath, 0755)
		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=/run/test", ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/run/test rw,nosuid,nodev,noexec,relatime - tmpfs"))
	})

	It("podman run with cidfile", func() {
		session := podmanTest.Podman([]string{"run", "--cidfile", tempdir + "cidfile", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		err := os.Remove(tempdir + "cidfile")
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

	It("podman run device-read-bps test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--device-read-bps=/dev/zero:1mb", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.throttle.read_bps_device"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("1048576"))
	})

	It("podman run device-write-bps test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--device-write-bps=/dev/zero:1mb", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.throttle.write_bps_device"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("1048576"))
	})

	It("podman run device-read-iops test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--device-read-iops=/dev/zero:100", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.throttle.read_iops_device"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("100"))
	})

	It("podman run device-write-iops test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--device-write-iops=/dev/zero:100", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.throttle.write_iops_device"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("100"))
	})

	It("podman run notify_socket", func() {
		sock := "/run/notify"
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
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman test hooks", func() {
		hcheck := "/run/hookscheck"
		hooksDir := tempdir + "/hooks"
		os.Mkdir(hooksDir, 0755)
		fileutils.CopyFile("hooks/hooks.json", hooksDir)
		os.Setenv("HOOK_OPTION", fmt.Sprintf("--hooks-dir-path=%s", hooksDir))
		os.Remove(hcheck)
		session := podmanTest.Podman([]string{"run", ALPINE, "ls"})
		session.Wait(10)
		os.Unsetenv("HOOK_OPTION")
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run with secrets", func() {
		containersDir := filepath.Join(podmanTest.TempDir, "containers")
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

		targetDir := tempdir + "/symlink/target"
		err = os.MkdirAll(targetDir, 0755)
		Expect(err).To(BeNil())
		keyFile := filepath.Join(targetDir, "key.pem")
		err = ioutil.WriteFile(keyFile, []byte(mountString), 0755)
		Expect(err).To(BeNil())
		execSession := SystemExec("ln", []string{"-s", targetDir, filepath.Join(secretsDir, "mysymlink")})
		execSession.WaitWithDefaultTimeout()
		Expect(execSession.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"--default-mounts-file=" + mountsFile, "run", "--rm", ALPINE, "cat", "/run/secrets/test.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal(secretsString))

		session = podmanTest.Podman([]string{"--default-mounts-file=" + mountsFile, "run", "--rm", ALPINE, "ls", "/run/secrets/mysymlink"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("key.pem"))
	})

	It("podman run with FIPS mode secrets", func() {
		fipsFile := "/etc/system-fips"
		err = ioutil.WriteFile(fipsFile, []byte{}, 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "ls", "/run/secrets"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("system-fips"))

		err = os.Remove(fipsFile)
		Expect(err).To(BeNil())
	})

	It("podman run without group-add", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("uid=0(root) gid=0(root) groups=0(root),1(bin),2(daemon),3(sys),4(adm),6(disk),10(wheel),11(floppy),20(dialout),26(tape),27(video)"))
	})

	It("podman run with group-add", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--group-add=audio", "--group-add=nogroup", "--group-add=777", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("uid=0(root) gid=0(root) groups=0(root),1(bin),2(daemon),3(sys),4(adm),6(disk),10(wheel),11(floppy),18(audio),20(dialout),26(tape),27(video),777,65533(nogroup)"))
	})

	It("podman run with user (default)", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("uid=0(root) gid=0(root) groups=0(root),1(bin),2(daemon),3(sys),4(adm),6(disk),10(wheel),11(floppy),20(dialout),26(tape),27(video)"))
	})

	It("podman run with user (integer, not in /etc/passwd)", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=1234", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("uid=1234(1234) gid=0(root)"))
	})

	It("podman run with user (integer, in /etc/passwd)", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=8", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("uid=8(mail) gid=12(mail) groups=12(mail)"))
	})

	It("podman run with user (username)", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=mail", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("uid=8(mail) gid=12(mail) groups=12(mail)"))
	})

	It("podman run with user:group (username:integer)", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=mail:21", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("uid=8(mail) gid=21(ftp)"))
	})

	It("podman run with user:group (integer:groupname)", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=8:ftp", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("uid=8(mail) gid=21(ftp)"))
	})

	It("podman run with user, verify caps dropped", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=1234", ALPINE, "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		capEff := strings.Split(session.OutputToString(), " ")
		Expect("0000000000000000").To(Equal(capEff[1]))
	})

	It("podman run with attach stdin outputs container ID", func() {
		session := podmanTest.Podman([]string{"run", "--attach", "stdin", ALPINE, "printenv"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ps := podmanTest.Podman([]string{"ps", "-aq", "--no-trunc"})
		ps.WaitWithDefaultTimeout()
		Expect(ps.ExitCode()).To(Equal(0))
		Expect(ps.LineInOutputContains(session.OutputToString())).To(BeTrue())
	})

	It("podman run with attach stdout does not print stderr", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--attach", "stdout", ALPINE, "ls", "/doesnotexist"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Equal(""))
	})

	It("podman run with attach stderr does not print stdout", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--attach", "stderr", ALPINE, "ls", "/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal(""))
	})

	It("podman run attach nonsense errors", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--attach", "asdfasdf", ALPINE, "ls", "/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman run exit code on failure to exec", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "/etc"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(126))
	})

	It("podman run error on exec", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "sh", "-c", "exit 100"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(100))
	})

	It("podman run with built-in volume image", func() {
		podmanTest.RestoreArtifact(redis)
		session := podmanTest.Podman([]string{"run", "--rm", redis, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", redis})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		dockerfile := `FROM busybox
RUN mkdir -p /myvol/data && chown -R mail.0 /myvol
VOLUME ["/myvol/data"]
USER mail`

		podmanTest.BuildImage(dockerfile, "test", "false")
		session = podmanTest.Podman([]string{"run", "--rm", "test", "ls", "-al", "/myvol/data"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("mail root"))

		session = podmanTest.Podman([]string{"rmi", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run --volumes-from flag", func() {
		vol := filepath.Join(podmanTest.TempDir, "vol-test")
		err := os.MkdirAll(vol, 0755)
		Expect(err).To(BeNil())

		volFile := filepath.Join(vol, "test.txt")
		data := "Testing --volumes-from!!!"
		err = ioutil.WriteFile(volFile, []byte(data), 0755)
		Expect(err).To(BeNil())

		podmanTest.RestoreArtifact(redis)
		session := podmanTest.Podman([]string{"create", "--volume", vol + ":/myvol", redis, "sh"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ctrID := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--volumes-from", ctrID, ALPINE, "echo", "'testing read-write!' >> myvol/test.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--volumes-from", ctrID + ":z", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run --volumes-from flag with built-in volumes", func() {
		podmanTest.RestoreArtifact(redis)
		session := podmanTest.Podman([]string{"create", redis, "sh"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ctrID := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--volumes-from", ctrID, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("data"))

	})

	It("podman run --volumes flag with multiple volumes", func() {
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).To(BeNil())
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")
		err = os.MkdirAll(vol2, 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"run", "--volume", vol1 + ":/myvol1:z", "--volume", vol2 + ":/myvol2:z", ALPINE, "touch", "/myvol2/foo.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run --mount flag with multiple mounts", func() {
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).To(BeNil())
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")
		err = os.MkdirAll(vol2, 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"run", "--mount", "type=bind,src=" + vol1 + ",target=/myvol1,z", "--mount", "type=bind,src=" + vol2 + ",target=/myvol2,z", ALPINE, "touch", "/myvol2/foo.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run findmnt nothing shared", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).To(BeNil())
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")
		err = os.MkdirAll(vol2, 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"run", "--volume", vol1 + ":/myvol1:z", "--volume", vol2 + ":/myvol2:z", fedoraMinimal, "findmnt", "-o", "TARGET,PROPAGATION"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("shared")
		Expect(match).Should(BeFalse())
	})

	It("podman run findmnt shared", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).To(BeNil())
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")
		err = os.MkdirAll(vol2, 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"run", "--volume", vol1 + ":/myvol1:z", "--volume", vol2 + ":/myvol2:shared,z", fedoraMinimal, "findmnt", "-o", "TARGET,PROPAGATION"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("shared")
		Expect(match).Should(BeTrue())
	})
})
