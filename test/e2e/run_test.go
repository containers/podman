// +build !remoteclient

package integration

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/libpod/pkg/cgroups"
	. "github.com/containers/libpod/test/utils"
	"github.com/containers/storage/pkg/stringid"
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
		podmanTest.Setup()
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman run a container based on local image", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run a container based on a complex local image name", func() {
		SkipIfRootless()
		imageName := strings.TrimPrefix(nginx, "quay.io/")
		session := podmanTest.Podman([]string{"run", imageName, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ErrorToString()).ToNot(ContainSubstring("Trying to pull"))
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run a container based on on a short name with localhost", func() {
		tag := podmanTest.Podman([]string{"tag", nginx, "localhost/libpod/alpine_nginx:latest"})
		tag.WaitWithDefaultTimeout()

		rmi := podmanTest.Podman([]string{"rmi", nginx})
		rmi.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"run", "libpod/alpine_nginx:latest", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ErrorToString()).ToNot(ContainSubstring("Trying to pull"))
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman container run a container based on on a short name with localhost", func() {
		tag := podmanTest.Podman([]string{"image", "tag", nginx, "localhost/libpod/alpine_nginx:latest"})
		tag.WaitWithDefaultTimeout()

		rmi := podmanTest.Podman([]string{"image", "rm", nginx})
		rmi.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"container", "run", "libpod/alpine_nginx:latest", "ls"})
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

	It("podman run a container with a --rootfs", func() {
		rootfs := filepath.Join(tempdir, "rootfs")
		uls := filepath.Join("/", "usr", "local", "share")
		uniqueString := stringid.GenerateNonCryptoID()
		testFilePath := filepath.Join(uls, uniqueString)
		tarball := filepath.Join(tempdir, "rootfs.tar")

		err := os.Mkdir(rootfs, 0770)
		Expect(err).Should(BeNil())

		// Change image in predictable way to validate export
		csession := podmanTest.Podman([]string{"run", "--name", uniqueString, ALPINE,
			"/bin/sh", "-c", fmt.Sprintf("echo %s > %s", uniqueString, testFilePath)})
		csession.WaitWithDefaultTimeout()
		Expect(csession.ExitCode()).To(Equal(0))

		// Export from working container image guarantees working root
		esession := podmanTest.Podman([]string{"export", "--output", tarball, uniqueString})
		esession.WaitWithDefaultTimeout()
		Expect(esession.ExitCode()).To(Equal(0))
		Expect(tarball).Should(BeARegularFile())

		// N/B: This will loose any extended attributes like SELinux types
		fmt.Fprintf(os.Stderr, "Extracting container root tarball\n")
		tarsession := SystemExec("tar", []string{"xf", tarball, "-C", rootfs})
		Expect(tarsession.ExitCode()).To(Equal(0))
		Expect(filepath.Join(rootfs, uls)).Should(BeADirectory())

		// Other tests confirm SELinux types, just confirm --rootfs is working.
		session := podmanTest.Podman([]string{"run", "-i", "--security-opt", "label=disable",
			"--rootfs", rootfs, "cat", testFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Validate changes made in original container and export
		stdoutLines := session.OutputToStringArray()
		Expect(stdoutLines).Should(HaveLen(1))
		Expect(stdoutLines[0]).Should(Equal(uniqueString))
	})

	It("podman run a container with --init", func() {
		session := podmanTest.Podman([]string{"run", "--init", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run a container with --init and --init-path", func() {
		session := podmanTest.Podman([]string{"run", "--init", "--init-path", "/usr/libexec/podman/catatonit", ALPINE, "ls"})
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
		Expect(session).To(ExitWithError())
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
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "printenv", "HOME"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("/root")
		Expect(match).Should(BeTrue())

		session = podmanTest.Podman([]string{"run", "--rm", "--user", "2", ALPINE, "printenv", "HOME"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ = session.GrepString("/sbin")
		Expect(match).Should(BeTrue())

		session = podmanTest.Podman([]string{"run", "--rm", "--env", "HOME=/foo", ALPINE, "printenv", "HOME"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ = session.GrepString("/foo")
		Expect(match).Should(BeTrue())

		session = podmanTest.Podman([]string{"run", "--rm", "--env", "FOO=BAR,BAZ", ALPINE, "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ = session.GrepString("BAR,BAZ")
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

		session = podmanTest.Podman([]string{"run", "--rm", "--env", "FOO", ALPINE, "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(len(session.OutputToString())).To(Equal(0))
		Expect(session.ExitCode()).To(Equal(1))

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

	It("podman run --host-env environment test", func() {
		os.Setenv("FOO", "BAR")
		session := podmanTest.Podman([]string{"run", "--rm", "--env-host", ALPINE, "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("BAR")
		Expect(match).Should(BeTrue())

		session = podmanTest.Podman([]string{"run", "--rm", "--env", "FOO=BAR1", "--env-host", ALPINE, "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ = session.GrepString("BAR1")
		Expect(match).Should(BeTrue())
		os.Unsetenv("FOO")
	})

	It("podman run limits test", func() {
		SkipIfRootless()
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

		cgroupsv2, err := cgroups.IsCgroup2UnifiedMode()
		Expect(err).To(BeNil())

		if !cgroupsv2 {
			// --oom-kill-disable not supported on cgroups v2.
			session = podmanTest.Podman([]string{"run", "--rm", "--oom-kill-disable=true", fedoraMinimal, "echo", "memory-hog"})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))
		}

		session = podmanTest.Podman([]string{"run", "--rm", "--oom-score-adj=100", fedoraMinimal, "cat", "/proc/self/oom_score_adj"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("100"))
	})

	It("podman run limits host test", func() {
		SkipIfRemote()

		var l syscall.Rlimit

		err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &l)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"run", "--rm", "--ulimit", "host", fedoraMinimal, "ulimit", "-Hn"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		ulimitCtrStr := strings.TrimSpace(session.OutputToString())
		ulimitCtr, err := strconv.ParseUint(ulimitCtrStr, 10, 0)
		Expect(err).To(BeNil())

		Expect(ulimitCtr).Should(BeNumerically(">=", l.Max))
	})

	It("podman run with cidfile", func() {
		session := podmanTest.Podman([]string{"run", "--cidfile", tempdir + "cidfile", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		err := os.Remove(tempdir + "cidfile")
		Expect(err).To(BeNil())
	})

	It("podman run sysctl test", func() {
		SkipIfRootless()
		session := podmanTest.Podman([]string{"run", "--rm", "--sysctl", "net.core.somaxconn=65535", ALPINE, "sysctl", "net.core.somaxconn"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("net.core.somaxconn = 65535"))
	})

	It("podman run blkio-weight test", func() {
		SkipIfRootless()
		cgroupsv2, err := cgroups.IsCgroup2UnifiedMode()
		Expect(err).To(BeNil())

		if !cgroupsv2 {
			if _, err := os.Stat("/sys/fs/cgroup/blkio/blkio.weight"); os.IsNotExist(err) {
				Skip("Kernel does not support blkio.weight")
			}
		}

		if cgroupsv2 {
			// convert linearly from [10-1000] to [1-10000]
			session := podmanTest.Podman([]string{"run", "--rm", "--blkio-weight=15", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/io.bfq.weight"})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))
			Expect(session.OutputToString()).To(ContainSubstring("51"))
		} else {
			session := podmanTest.Podman([]string{"run", "--rm", "--blkio-weight=15", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.weight"})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))
			Expect(session.OutputToString()).To(ContainSubstring("15"))
		}
	})

	It("podman run device-read-bps test", func() {
		SkipIfRootless()

		cgroupsv2, err := cgroups.IsCgroup2UnifiedMode()
		Expect(err).To(BeNil())

		var session *PodmanSessionIntegration

		if cgroupsv2 {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-read-bps=/dev/zero:1mb", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/io.max"})
		} else {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-read-bps=/dev/zero:1mb", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.throttle.read_bps_device"})
		}

		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("1048576"))
	})

	It("podman run device-write-bps test", func() {
		SkipIfRootless()

		cgroupsv2, err := cgroups.IsCgroup2UnifiedMode()
		Expect(err).To(BeNil())

		var session *PodmanSessionIntegration

		if cgroupsv2 {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-write-bps=/dev/zero:1mb", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/io.max"})
		} else {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-write-bps=/dev/zero:1mb", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.throttle.write_bps_device"})
		}
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("1048576"))
	})

	It("podman run device-read-iops test", func() {
		SkipIfRootless()

		cgroupsv2, err := cgroups.IsCgroup2UnifiedMode()
		Expect(err).To(BeNil())

		var session *PodmanSessionIntegration

		if cgroupsv2 {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-read-iops=/dev/zero:100", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/io.max"})
		} else {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-read-iops=/dev/zero:100", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.throttle.read_iops_device"})
		}

		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("100"))
	})

	It("podman run device-write-iops test", func() {
		SkipIfRootless()

		cgroupsv2, err := cgroups.IsCgroup2UnifiedMode()
		Expect(err).To(BeNil())

		var session *PodmanSessionIntegration

		if cgroupsv2 {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-write-iops=/dev/zero:100", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/io.max"})
		} else {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-write-iops=/dev/zero:100", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.throttle.write_iops_device"})
		}

		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("100"))
	})

	It("podman run notify_socket", func() {
		SkipIfRemote()

		host := GetHostDistributionInfo()
		if host.Distribution != "rhel" && host.Distribution != "centos" && host.Distribution != "fedora" {
			Skip("this test requires a working runc")
		}
		sock := filepath.Join(podmanTest.TempDir, "notify")
		addr := net.UnixAddr{
			Name: sock,
			Net:  "unixgram",
		}
		socket, err := net.ListenUnixgram("unixgram", &addr)
		Expect(err).To(BeNil())
		defer os.Remove(sock)
		defer socket.Close()

		os.Setenv("NOTIFY_SOCKET", sock)
		defer os.Unsetenv("NOTIFY_SOCKET")

		session := podmanTest.Podman([]string{"run", ALPINE, "printenv", "NOTIFY_SOCKET"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 0))
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
		podmanTest.RestoreArtifact(BB)
		tag := podmanTest.PodmanNoCache([]string{"tag", "busybox", "bb"})
		tag.WaitWithDefaultTimeout()
		Expect(tag.ExitCode()).To(Equal(0))

		session := podmanTest.PodmanNoCache([]string{"run", "--rm", "bb", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman test hooks", func() {
		hcheck := "/run/hookscheck"
		hooksDir := tempdir + "/hooks"
		os.Mkdir(hooksDir, 0755)
		fileutils.CopyFile("hooks/hooks.json", hooksDir)
		os.Setenv("HOOK_OPTION", fmt.Sprintf("--hooks-dir=%s", hooksDir))
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
		SkipIfRootless()
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
		SkipIfRootless()
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("uid=0(root) gid=0(root) groups=0(root),1(bin),2(daemon),3(sys),4(adm),6(disk),10(wheel),11(floppy),20(dialout),26(tape),27(video)"))
	})

	It("podman run with group-add", func() {
		SkipIfRootless()
		session := podmanTest.Podman([]string{"run", "--rm", "--group-add=audio", "--group-add=nogroup", "--group-add=777", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("uid=0(root) gid=0(root) groups=0(root),1(bin),2(daemon),3(sys),4(adm),6(disk),10(wheel),11(floppy),18(audio),20(dialout),26(tape),27(video),777,65533(nogroup)"))
	})

	It("podman run with user (default)", func() {
		SkipIfRootless()
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
		SkipIfRootless()
		session := podmanTest.Podman([]string{"run", "--rm", "--user=8", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("uid=8(mail) gid=12(mail) groups=12(mail)"))
	})

	It("podman run with user (username)", func() {
		SkipIfRootless()
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
		session := podmanTest.Podman([]string{"run", "--rm", redis, "ls"})
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
	})

	It("podman run --volumes-from flag", func() {
		vol := filepath.Join(podmanTest.TempDir, "vol-test")
		err := os.MkdirAll(vol, 0755)
		Expect(err).To(BeNil())

		volFile := filepath.Join(vol, "test.txt")
		data := "Testing --volumes-from!!!"
		err = ioutil.WriteFile(volFile, []byte(data), 0755)
		Expect(err).To(BeNil())

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

	It("podman run --volumes flag with empty host dir", func() {
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"run", "--volume", ":/myvol1:z", ALPINE, "touch", "/myvol2/foo.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("directory cannot be empty"))
		session = podmanTest.Podman([]string{"run", "--volume", vol1 + ":", ALPINE, "touch", "/myvol2/foo.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("directory cannot be empty"))
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
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).To(BeNil())
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")
		err = os.MkdirAll(vol2, 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"run", "--volume", vol1 + ":/myvol1:z", "--volume", vol2 + ":/myvol2:shared,z", fedoraMinimal, "findmnt", "-o", "TARGET,PROPAGATION"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, shared := session.GrepString("shared")
		Expect(match).Should(BeTrue())
		// make sure it's only shared (and not 'shared,slave')
		isSharedOnly := !strings.Contains(shared[0], "shared,")
		Expect(isSharedOnly).Should(BeTrue())
	})

	It("podman run --mount type=bind,bind-nonrecursive", func() {
		SkipIfRootless()
		session := podmanTest.Podman([]string{"run", "--mount", "type=bind,bind-nonrecursive,slave,src=/,target=/host", fedoraMinimal, "findmnt", "-nR", "/host"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(1))
	})

	It("podman run --pod automatically", func() {
		session := podmanTest.Podman([]string{"run", "--pod", "new:foobar", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		match, _ := check.GrepString("foobar")
		Expect(match).To(BeTrue())
	})

	It("podman run --rm should work", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		numContainers := podmanTest.NumberOfContainers()
		Expect(numContainers).To(Equal(0))
	})

	It("podman run --rm failed container should delete itself", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		numContainers := podmanTest.NumberOfContainers()
		Expect(numContainers).To(Equal(0))
	})

	It("podman run failed container should NOT delete itself", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		numContainers := podmanTest.NumberOfContainers()
		Expect(numContainers).To(Equal(1))
	})
	It("podman run readonly container should NOT mount /dev/shm read/only", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", ALPINE, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		Expect(session.OutputToString()).To(Not(ContainSubstring("/dev/shm type tmpfs (ro,")))
	})

	It("podman run with bad healthcheck retries", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--health-cmd", "[\"foo\"]", "--health-retries", "0", ALPINE, "top"})
		session.Wait()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("healthcheck-retries must be greater than 0"))
	})

	It("podman run with bad healthcheck timeout", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--health-cmd", "[\"foo\"]", "--health-timeout", "0s", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("healthcheck-timeout must be at least 1 second"))
	})

	It("podman run with bad healthcheck start-period", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--health-cmd", "[\"foo\"]", "--health-start-period", "-1s", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("healthcheck-start-period must be 0 seconds or greater"))
	})

	It("podman run with --add-host and --no-hosts fails", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--add-host", "test1:127.0.0.1", "--no-hosts", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run --http-proxy test", func() {
		os.Setenv("http_proxy", "1.2.3.4")
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "printenv", "http_proxy"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("1.2.3.4")
		Expect(match).Should(BeTrue())

		session = podmanTest.Podman([]string{"run", "--http-proxy=false", ALPINE, "printenv", "http_proxy"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(1))
		Expect(session.OutputToString()).To(Equal(""))
		os.Unsetenv("http_proxy")
	})

	It("podman run with restart-policy always restarts containers", func() {

		testDir := filepath.Join(podmanTest.RunRoot, "restart-test")
		err := os.MkdirAll(testDir, 0755)
		Expect(err).To(BeNil())

		aliveFile := filepath.Join(testDir, "running")
		file, err := os.Create(aliveFile)
		Expect(err).To(BeNil())
		file.Close()

		session := podmanTest.Podman([]string{"run", "-dt", "--restart", "always", "-v", fmt.Sprintf("%s:/tmp/runroot:Z", testDir), fedoraMinimal, "bash", "-c", "date +%N > /tmp/runroot/ran && while test -r /tmp/runroot/running; do sleep 0.1s; done"})

		found := false
		testFile := filepath.Join(testDir, "ran")
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			if _, err := os.Stat(testFile); err == nil {
				found = true
				err = os.Remove(testFile)
				Expect(err).To(BeNil())
				break
			}
		}
		Expect(found).To(BeTrue())

		err = os.Remove(aliveFile)
		Expect(err).To(BeNil())

		session.WaitWithDefaultTimeout()

		// 10 seconds to restart the container
		found = false
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			if _, err := os.Stat(testFile); err == nil {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue())
	})

	It("podman run with cgroups=disabled runs without cgroups", func() {
		SkipIfRemote()
		SkipIfRootless()
		// Only works on crun
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test only works on crun")
		}

		curCgroupsBytes, err := ioutil.ReadFile("/proc/self/cgroup")
		Expect(err).To(BeNil())
		var curCgroups string = string(curCgroupsBytes)
		fmt.Printf("Output:\n%s\n", curCgroups)
		Expect(curCgroups).To(Not(Equal("")))

		ctrName := "testctr"
		container := podmanTest.Podman([]string{"run", "--name", ctrName, "-d", "--cgroups=disabled", ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container.ExitCode()).To(Equal(0))

		// Get PID and get cgroups of that PID
		inspectOut := podmanTest.InspectContainer(ctrName)
		Expect(len(inspectOut)).To(Equal(1))
		pid := inspectOut[0].State.Pid
		Expect(pid).To(Not(Equal(0)))
		Expect(inspectOut[0].HostConfig.CgroupParent).To(Equal(""))

		ctrCgroupsBytes, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
		Expect(err).To(BeNil())
		var ctrCgroups string = string(ctrCgroupsBytes)
		fmt.Printf("Output\n:%s\n", ctrCgroups)
		Expect(curCgroups).To(Equal(ctrCgroups))
	})

	It("podman run with cgroups=enabled makes cgroups", func() {
		SkipIfRemote()
		SkipIfRootless()
		// Only works on crun
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test only works on crun")
		}

		curCgroupsBytes, err := ioutil.ReadFile("/proc/self/cgroup")
		Expect(err).To(BeNil())
		var curCgroups string = string(curCgroupsBytes)
		fmt.Printf("Output:\n%s\n", curCgroups)
		Expect(curCgroups).To(Not(Equal("")))

		ctrName := "testctr"
		container := podmanTest.Podman([]string{"run", "--name", ctrName, "-d", "--cgroups=enabled", ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container.ExitCode()).To(Equal(0))

		// Get PID and get cgroups of that PID
		inspectOut := podmanTest.InspectContainer(ctrName)
		Expect(len(inspectOut)).To(Equal(1))
		pid := inspectOut[0].State.Pid
		Expect(pid).To(Not(Equal(0)))

		ctrCgroupsBytes, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
		Expect(err).To(BeNil())
		var ctrCgroups string = string(ctrCgroupsBytes)
		fmt.Printf("Output\n:%s\n", ctrCgroups)
		Expect(curCgroups).To(Not(Equal(ctrCgroups)))
	})

	It("podman run with cgroups=garbage errors", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--cgroups=garbage", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run should fail with nonexist authfile", func() {
		SkipIfRemote()
		session := podmanTest.Podman([]string{"run", "--authfile", "/tmp/nonexist", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})
})
