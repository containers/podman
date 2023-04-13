package integration

/*
	toolbox_test.go is under the care of the Toolbox Team.

	The tests are trying to stress parts of Podman that Toolbox[0] needs for
	its functionality.

	[0] https://github.com/containers/toolbox

	Info about test cases:
	- some tests rely on a certain configuration of a container that is done by
		executing several commands in the entry-point of a container. To make
		sure the initialization had enough time to be executed,
		WaitContainerReady() after the container is started.

	- in several places there's an invocation of 'podman logs' It is there mainly
		to ease debugging when a test goes wrong (during the initialization of a
		container) but sometimes it is also used in the test case itself.

	Maintainers (Toolbox Team):
	- Ondřej Míchal <harrymichal@fedoraproject.org>
	- Debarshi Ray <rishi@fedoraproject.org>

	Also available on Freenode IRC on #silverblue or #podman
*/

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Toolbox-specific testing", func() {
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentSpecReport()
		processTestResult(f)
	})

	It("podman run --dns=none - allows self-management of /etc/resolv.conf", func() {
		session := podmanTest.Podman([]string{"run", "--dns", "none", ALPINE, "sh", "-c",
			"rm -f /etc/resolv.conf; touch -d '1970-01-01 00:02:03' /etc/resolv.conf; stat -c %s:%Y /etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("0:123"))
	})

	It("podman run --no-hosts - allows self-management of /etc/hosts", func() {
		session := podmanTest.Podman([]string{"run", "--no-hosts", ALPINE, "sh", "-c",
			"rm -f /etc/hosts; touch -d '1970-01-01 00:02:03' /etc/hosts; stat -c %s:%Y /etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("0:123"))
	})

	It("podman create --ulimit host + podman exec - correctly mirrors hosts ulimits", func() {
		if podmanTest.RemoteTest {
			Skip("Ulimit check does not work with a remote client")
		}
		var session *PodmanSessionIntegration
		var containerHardLimit int
		var rlimit syscall.Rlimit
		var err error

		err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit)
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Expected value: %d", rlimit.Max)

		session = podmanTest.Podman([]string{"create", "--name", "test", "--ulimit", "host", ALPINE,
			"sleep", "1000"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "test", "sh", "-c",
			"ulimit -H -n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		containerHardLimit, err = strconv.Atoi(strings.Trim(session.OutputToString(), "\n"))
		Expect(err).ToNot(HaveOccurred())
		Expect(containerHardLimit).To(BeNumerically(">=", rlimit.Max))
	})

	It("podman create --ipc=host --pid=host + podman exec - correct shared memory limit size", func() {
		// Comparison of the size of /dev/shm on the host being equal to the one in
		// a container
		if podmanTest.RemoteTest {
			Skip("Shm size check does not work with a remote client")
		}
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		var session *PodmanSessionIntegration
		var cmd *exec.Cmd
		var hostShmSize, containerShmSize int
		var err error

		// Because Alpine uses busybox, most commands don't offer advanced options
		// like "--output" in df. Therefore the value of the field 'Size' (or
		// ('1K-blocks') needs to be extracted manually.
		cmd = exec.Command("df", "/dev/shm")
		res, err := cmd.Output()
		Expect(err).ToNot(HaveOccurred())
		lines := strings.SplitN(string(res), "\n", 2)
		fields := strings.Fields(lines[len(lines)-1])
		hostShmSize, err = strconv.Atoi(fields[1])
		Expect(err).ToNot(HaveOccurred())

		session = podmanTest.Podman([]string{"create", "--name", "test", "--ipc=host", "--pid=host", ALPINE,
			"sleep", "1000"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "test",
			"df", "/dev/shm"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		lines = session.OutputToStringArray()
		fields = strings.Fields(lines[len(lines)-1])
		containerShmSize, err = strconv.Atoi(fields[1])
		Expect(err).ToNot(HaveOccurred())

		// In some cases it may happen that the size of /dev/shm is not exactly
		// equal. Therefore it's fine if there's a slight tolerance between the
		// compared values.
		Expect(hostShmSize).To(BeNumerically("~", containerShmSize, 100))
	})

	It("podman create --userns=keep-id --user root:root - entrypoint - entrypoint is executed as root", func() {
		SkipIfNotRootless("only meaningful when run rootless")
		session := podmanTest.Podman([]string{"run", "--userns=keep-id", "--user", "root:root", ALPINE,
			"id"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("uid=0(root) gid=0(root)"))
	})

	It("podman create --userns=keep-id + podman exec - correct names of user and group", func() {
		SkipIfNotRootless("only meaningful when run rootless")
		var session *PodmanSessionIntegration
		var err error

		currentUser, err := user.Current()
		Expect(err).ToNot(HaveOccurred())

		currentGroup, err := user.LookupGroupId(currentUser.Gid)
		Expect(err).ToNot(HaveOccurred())

		session = podmanTest.Podman([]string{"create", "--name", "test", "--userns=keep-id", ALPINE,
			"sleep", "1000"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(err).ToNot(HaveOccurred())

		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		expectedOutput := fmt.Sprintf("uid=%s(%s) gid=%s(%s)",
			currentUser.Uid, currentUser.Username,
			currentGroup.Gid, currentGroup.Name)

		session = podmanTest.Podman([]string{"exec", "test",
			"id"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(expectedOutput))
	})

	It("podman create --userns=keep-id - entrypoint - adding user with useradd and then removing their password", func() {
		SkipIfNotRootless("only meaningful when run rootless")
		var session *PodmanSessionIntegration

		var username string = "testuser"
		var homeDir string = "/home/testuser"
		var shell string = "/bin/sh"
		var uid string = "1001"
		var gid string = "1001"

		useradd := fmt.Sprintf("useradd --home-dir %s --shell %s --uid %s %s",
			homeDir, shell, uid, username)
		passwd := fmt.Sprintf("passwd --delete %s", username)
		session = podmanTest.Podman([]string{"create", "--log-driver", "k8s-file", "--name", "test", "--userns=keep-id", "--user", "root:root", fedoraToolbox, "sh", "-c",
			fmt.Sprintf("%s; %s; echo READY; sleep 1000", useradd, passwd)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		Expect(WaitContainerReady(podmanTest, "test", "READY", 5, 1)).To(BeTrue())

		expectedOutput := fmt.Sprintf("%s:x:%s:%s::%s:%s",
			username, uid, gid, homeDir, shell)

		session = podmanTest.Podman([]string{"exec", "test", "cat", "/etc/passwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(expectedOutput))

		expectedOutput = "passwd: Note: deleting a password also unlocks the password."

		session = podmanTest.Podman([]string{"logs", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(ContainSubstring(expectedOutput))
	})

	It("podman create --userns=keep-id + podman exec - adding group with groupadd", func() {
		SkipIfNotRootless("only meaningful when run rootless")
		var session *PodmanSessionIntegration

		var groupName string = "testgroup"
		var gid string = "1001"

		groupadd := fmt.Sprintf("groupadd --gid %s %s", gid, groupName)

		session = podmanTest.Podman([]string{"create", "--log-driver", "k8s-file", "--name", "test", "--userns=keep-id", "--user", "root:root", fedoraToolbox, "sh", "-c",
			fmt.Sprintf("%s; echo READY; sleep 1000", groupadd)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		Expect(WaitContainerReady(podmanTest, "test", "READY", 5, 1)).To(BeTrue())

		session = podmanTest.Podman([]string{"exec", "test", "cat", "/etc/group"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(groupName))

		session = podmanTest.Podman([]string{"logs", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("READY"))
	})

	It("podman create --userns=keep-id - entrypoint - modifying existing user with usermod - add to new group, change home/shell/uid", func() {
		SkipIfNotRootless("only meaningful when run rootless")
		var session *PodmanSessionIntegration
		var badHomeDir string = "/home/badtestuser"
		var badShell string = "/bin/sh"
		var badUID string = "1001"
		var username string = "testuser"
		var homeDir string = "/home/testuser"
		var shell string = "/bin/bash"
		var uid string = "2000"
		var groupName string = "testgroup"
		var gid string = "2000"

		// The use of bad* in the name of variables does not imply the invocation
		// of useradd should fail The user is supposed to be created successfully
		// but later his information (uid, home, shell,..) is changed via usermod.
		useradd := fmt.Sprintf("useradd --home-dir %s --shell %s --uid %s %s",
			badHomeDir, badShell, badUID, username)
		groupadd := fmt.Sprintf("groupadd --gid %s %s",
			gid, groupName)
		usermod := fmt.Sprintf("usermod --append --groups wheel --home %s --shell %s --uid %s --gid %s %s",
			homeDir, shell, uid, gid, username)

		session = podmanTest.Podman([]string{"create", "--log-driver", "k8s-file", "--name", "test", "--userns=keep-id", "--user", "root:root", fedoraToolbox, "sh", "-c",
			fmt.Sprintf("%s; %s; %s; echo READY; sleep 1000", useradd, groupadd, usermod)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		Expect(WaitContainerReady(podmanTest, "test", "READY", 5, 1)).To(BeTrue())

		expectedUser := fmt.Sprintf("%s:x:%s:%s::%s:%s",
			username, uid, gid, homeDir, shell)

		session = podmanTest.Podman([]string{"exec", "test", "cat", "/etc/passwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(expectedUser))

		session = podmanTest.Podman([]string{"logs", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("READY"))
	})

	It("podman run --privileged --userns=keep-id --user root:root - entrypoint - (bind)mounting", func() {
		SkipIfNotRootless("only meaningful when run rootless")
		var session *PodmanSessionIntegration

		session = podmanTest.Podman([]string{"run", "--privileged", "--userns=keep-id", "--user", "root:root", ALPINE,
			"mount", "-t", "tmpfs", "tmpfs", "/tmp"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--privileged", "--userns=keep-id", "--user", "root:root", ALPINE,
			"mount", "--rbind", "/tmp", "/var/tmp"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman create + start - with all needed switches for create - sleep as entry-point", func() {
		SkipIfNotRootless("only meaningful when run rootless")
		var session *PodmanSessionIntegration

		// These should be most of the switches that Toolbox uses to create a "toolbox" container
		// https://github.com/containers/toolbox/blob/main/src/cmd/create.go
		session = podmanTest.Podman([]string{"create",
			"--log-driver", "k8s-file",
			"--dns", "none",
			"--hostname", "toolbox",
			"--ipc", "host",
			"--label", "com.github.containers.toolbox=true",
			"--name", "test",
			"--network", "host",
			"--no-hosts",
			"--pid", "host",
			"--privileged",
			"--security-opt", "label=disable",
			"--ulimit", "host",
			"--userns=keep-id",
			"--user", "root:root",
			fedoraToolbox, "sh", "-c", "echo READY; sleep 1000"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		Expect(WaitContainerReady(podmanTest, "test", "READY", 5, 1)).To(BeTrue())

		session = podmanTest.Podman([]string{"logs", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("READY"))
	})

	It("podman run --userns=keep-id check $HOME", func() {
		SkipIfNotRootless("only meaningful when run rootless")
		var session *PodmanSessionIntegration
		currentUser, err := user.Current()
		Expect(err).ToNot(HaveOccurred())

		session = podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:%s", currentUser.HomeDir, currentUser.HomeDir), "--userns=keep-id", fedoraToolbox, "sh", "-c", "echo $HOME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(currentUser.HomeDir))

		if isRootless() {
			location := path.Dir(currentUser.HomeDir)
			volumeArg := fmt.Sprintf("%s:%s", location, location)
			session = podmanTest.Podman([]string{"run",
				"--userns=keep-id",
				"--volume", volumeArg,
				fedoraToolbox, "sh", "-c", "echo $HOME"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.OutputToString()).To(ContainSubstring(currentUser.HomeDir))
		}
	})

})
