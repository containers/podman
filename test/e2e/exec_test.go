//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman exec", func() {

	It("podman exec into bogus container", func() {
		session := podmanTest.Podman([]string{"exec", "foobar", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `no container with name or ID "foobar" found: no such container`))
	})

	It("podman exec simple command", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"exec", "test1", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// With no command
		session = podmanTest.Podman([]string{"exec", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "must provide a non-empty command to start an exec session: invalid argument"))
	})

	It("podman container exec simple command", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"container", "exec", "test1", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman exec simple command using latest", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cid := "-l"
		if IsRemote() {
			cid = "test1"
		}
		session := podmanTest.Podman([]string{"exec", cid, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman exec environment test", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"exec", "--env", "FOO=BAR", "test1", "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("BAR"))

		session = podmanTest.Podman([]string{"exec", "--env", "PATH=/bin", "test1", "printenv", "PATH"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("/bin"))
	})

	It("podman exec os.Setenv env", func() {
		// remote doesn't properly interpret os.Setenv
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		os.Setenv("FOO", "BAR")
		session := podmanTest.Podman([]string{"exec", "--env", "FOO", "test1", "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("BAR"))
		os.Unsetenv("FOO")
	})

	It("podman exec exit code", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"exec", "test1", "sh", "-c", "exit 100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(100, ""))
	})

	It("podman exec in keep-id container drops privileges", func() {
		SkipIfNotRootless("This function is not enabled for rootful podman")
		ctrName := "testctr1"
		testCtr := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, "--userns=keep-id", ALPINE, "top"})
		testCtr.WaitWithDefaultTimeout()
		Expect(testCtr).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"exec", ctrName, "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000000"))
	})

	It("podman exec --privileged", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		bndPerms := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--privileged", "--rm", ALPINE, "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		effPerms := session.OutputToString()

		setup := podmanTest.RunTopContainer("test-privileged")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "--privileged", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))

		session = podmanTest.Podman([]string{"exec", "--privileged", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))
	})

	It("podman exec --privileged", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "--user=bin", "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		bndPerms := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--privileged", "--user=bin", "--rm", ALPINE, "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		effPerms := session.OutputToString()

		setup := podmanTest.RunTopContainer("test-privileged")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=bin", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=bin", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))

	})

	It("podman exec --privileged", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		bndPerms := session.OutputToString()

		setup := podmanTest.RunTopContainer("test-privileged")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=bin", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("00000000"))

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=bin", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))
	})

	It("podman exec --privileged container not running as root", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		bndPerms := session.OutputToString()

		setup := podmanTest.RunTopContainerWithArgs("test-privileged", []string{"--user=bin"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "--privileged", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("00000000"))

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=bin", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("00000000"))

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=root", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=bin", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))
	})

	It("podman exec with user with cap-add", func() {
		capAdd := "--cap-add=net_bind_service"
		session := podmanTest.Podman([]string{"run", "--user=bin", capAdd, "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		bndPerms := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--user=bin", capAdd, "--rm", ALPINE, "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		effPerms := session.OutputToString()

		setup := podmanTest.RunTopContainerWithArgs("test-privileged", []string{"--user=bin", capAdd})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))

		session = podmanTest.Podman([]string{"exec", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))
	})

	It("podman exec with user with and cap-drop cap-add", func() {
		capAdd := "--cap-add=net_bind_service"
		capDrop := "--cap-drop=all"
		session := podmanTest.Podman([]string{"run", "--user=bin", capDrop, capAdd, "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		bndPerms := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--user=bin", capDrop, capAdd, "--rm", ALPINE, "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		effPerms := session.OutputToString()

		setup := podmanTest.RunTopContainerWithArgs("test-privileged", []string{"--user=bin", capDrop, capAdd})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))

		session = podmanTest.Podman([]string{"exec", "--privileged", "test-privileged", "sh", "-c", "grep ^CapInh /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))

		session = podmanTest.Podman([]string{"exec", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))

		session = podmanTest.Podman([]string{"exec", "test-privileged", "sh", "-c", "grep ^CapPrm /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))

		session = podmanTest.Podman([]string{"exec", "test-privileged", "sh", "-c", "grep ^CapAmb /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))
	})

	It("podman exec --privileged with user", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "--user=bin", "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		bindPerms := session.OutputToString()

		setup := podmanTest.RunTopContainerWithArgs("test-privileged", []string{"--privileged", "--user=bin"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "--privileged", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(bindPerms))

		session = podmanTest.Podman([]string{"exec", "--privileged", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000000"))
	})

	// #10927 ("no logs from conmon"), one of our nastiest flakes
	It("podman exec terminal doesn't hang", FlakeAttempts(3), func() {
		setup := podmanTest.Podman([]string{"run", "-dti", "--name", "test1", fedoraMinimal, "sleep", "+Inf"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))
		Expect(setup.ErrorToString()).To(ContainSubstring("The input device is not a TTY. The --tty and --interactive flags might not work properly"))

		for i := 0; i < 5; i++ {
			session := podmanTest.Podman([]string{"exec", "-ti", "test1", "true"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
		}
	})

	It("podman exec pseudo-terminal sanity check", func() {
		setup := podmanTest.Podman([]string{"run", "--detach", "--name", "test1", fedoraMinimal, "sleep", "+Inf"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"exec", "--interactive", "--tty", "test1", "/usr/bin/stty", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(" onlcr"))
	})

	It("podman exec simple command with user", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"exec", "--user", "root", "test1", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman exec with user only in container", func() {
		testUser := "test123"
		setup := podmanTest.Podman([]string{"run", "--name", "test1", "-d", CITEST_IMAGE, "sleep", "60"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"exec", "test1", "adduser", "-D", testUser})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session2 := podmanTest.Podman([]string{"exec", "--user", testUser, "test1", "whoami"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
		Expect(session2.OutputToString()).To(Equal(testUser))
	})

	It("podman exec with user from run", func() {
		testUser := "guest"
		setup := podmanTest.Podman([]string{"run", "--user", testUser, "-d", ALPINE, "top"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		ctrID := setup.OutputToString()

		session := podmanTest.Podman([]string{"exec", ctrID, "whoami"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(testUser))

		overrideUser := "root"
		session = podmanTest.Podman([]string{"exec", "--user", overrideUser, ctrID, "whoami"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(overrideUser))
	})

	It("podman exec simple working directory test", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"exec", "--workdir", "/tmp", "test1", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("/tmp"))

		session = podmanTest.Podman([]string{"exec", "-w", "/tmp", "test1", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("/tmp"))
	})

	It("podman exec missing working directory test", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		expect := "chdir to `/missing`: No such file or directory"
		if podmanTest.OCIRuntime == "runc" {
			expect = "chdir to cwd"
		}
		session := podmanTest.Podman([]string{"exec", "--workdir", "/missing", "test1", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(127, expect))

		session = podmanTest.Podman([]string{"exec", "-w", "/missing", "test1", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(127, expect))
	})

	It("podman exec cannot be invoked", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"exec", "test1", "/etc"})
		session.WaitWithDefaultTimeout()

		// crun (and, we hope, any other future runtimes)
		expectedStatus := 126
		expectedMessage := "open executable: Operation not permitted: OCI permission denied"

		// ...but it's much more complicated under runc (#19552)
		if podmanTest.OCIRuntime == "runc" {
			expectedMessage = `exec failed: unable to start container process: exec: "/etc": is a directory`
			expectedStatus = 255
			if IsRemote() {
				expectedStatus = 125
			}
		}
		Expect(session).Should(ExitWithError(expectedStatus, expectedMessage))
	})

	It("podman exec command not found", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"exec", "test1", "notthere"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(127, "OCI runtime attempted to invoke a command that was not found"))
	})

	It("podman exec preserve fds sanity check", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		devNull, err := os.Open("/dev/null")
		Expect(err).ToNot(HaveOccurred())
		defer devNull.Close()
		files := []*os.File{
			devNull,
		}
		session := podmanTest.PodmanExtraFiles([]string{"exec", "--preserve-fds", "1", "test1", "ls"}, files)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman exec preserves --group-add groups", func() {
		groupName := "group1"
		gid := "4444"
		ctrName1 := "ctr1"
		ctr1 := podmanTest.Podman([]string{"run", "--name", ctrName1, fedoraMinimal, "groupadd", "-g", gid, groupName})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())

		imgName := "img1"
		commit := podmanTest.Podman([]string{"commit", "-q", ctrName1, imgName})
		commit.WaitWithDefaultTimeout()
		Expect(commit).Should(ExitCleanly())

		ctrName2 := "ctr2"
		ctr2 := podmanTest.Podman([]string{"run", "-d", "--name", ctrName2, "--group-add", groupName, imgName, "sleep", "300"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(ExitCleanly())

		exec := podmanTest.Podman([]string{"exec", ctrName2, "id"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())
		Expect(exec.OutputToString()).To(ContainSubstring(fmt.Sprintf("%s(%s)", gid, groupName)))
	})

	It("podman exec preserves container groups with --user and --group-add", func() {
		dockerfile := fmt.Sprintf(`FROM %s
RUN groupadd -g 4000 first
RUN groupadd -g 4001 second
RUN useradd -u 1000 auser`, fedoraMinimal)
		imgName := "testimg"
		podmanTest.BuildImage(dockerfile, imgName, "false")

		ctrName := "testctr"
		ctr := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, "--user", "auser:first", "--group-add", "second", imgName, "sleep", "300"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		exec := podmanTest.Podman([]string{"exec", ctrName, "id"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())
		output := exec.OutputToString()
		Expect(output).To(ContainSubstring("4000(first)"))
		Expect(output).To(ContainSubstring("4001(second)"))
		Expect(output).To(ContainSubstring("1000(auser)"))

		// Kill the container just so the test does not take 15 seconds to stop.
		kill := podmanTest.Podman([]string{"kill", ctrName})
		kill.WaitWithDefaultTimeout()
		Expect(kill).Should(ExitCleanly())
	})

	It("podman exec --detach", func() {
		ctrName := "testctr"
		ctr := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		exec1 := podmanTest.Podman([]string{"exec", "-d", ctrName, "top"})
		exec1.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		data := podmanTest.InspectContainer(ctrName)
		Expect(data).To(HaveLen(1))
		Expect(data[0].ExecIDs).To(HaveLen(1))
		Expect(exec1.OutputToString()).To(ContainSubstring(data[0].ExecIDs[0]))

		exec2 := podmanTest.Podman([]string{"exec", ctrName, "ps", "-a"})
		exec2.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())
		Expect(strings.Count(exec2.OutputToString(), "top")).To(Equal(2))

		// Ensure that stop with a running detached exec session is
		// clean.
		podmanTest.StopContainer(ctrName)
	})

	It("podman exec with env var secret", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "-d", "--secret", "source=mysecret,type=env", "--name", "secr", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "secr", "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(secretsString))

		session = podmanTest.Podman([]string{"commit", "-q", "secr", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "foobar.com/test1-image:latest", "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Not(ContainSubstring(secretsString)))
	})

	It("podman exec --wait 2 seconds on bogus container", func() {
		SkipIfRemote("not supported for --wait")
		session := podmanTest.Podman([]string{"exec", "--wait", "2", "1234"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "timed out waiting for container: 1234"))
	})

	It("podman exec --wait 5 seconds for started container", func() {
		SkipIfRemote("not supported for --wait")
		ctrName := "waitCtr"

		session := podmanTest.Podman([]string{"exec", "--wait", "5", ctrName, "whoami"})

		session2 := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, ALPINE, "top"})
		session2.WaitWithDefaultTimeout()

		session.Wait(6)

		Expect(session2).Should(ExitCleanly())
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("root"))
	})
})
