package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman exec", func() {
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

	It("podman exec into bogus container", func() {
		session := podmanTest.Podman([]string{"exec", "foobar", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman exec without command", func() {
		session := podmanTest.Podman([]string{"exec", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman exec simple command", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "test1", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman container exec simple command", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"container", "exec", "test1", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman exec simple command using latest", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))
		cid := "-l"
		if IsRemote() {
			cid = "test1"
		}
		session := podmanTest.Podman([]string{"exec", cid, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman exec environment test", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "--env", "FOO=BAR", "test1", "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("BAR"))

		session = podmanTest.Podman([]string{"exec", "--env", "PATH=/bin", "test1", "printenv", "PATH"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("/bin"))
	})

	It("podman exec os.Setenv env", func() {
		// remote doesn't properly interpret os.Setenv
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		os.Setenv("FOO", "BAR")
		session := podmanTest.Podman([]string{"exec", "--env", "FOO", "test1", "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("BAR"))
		os.Unsetenv("FOO")
	})

	It("podman exec exit code", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "test1", "sh", "-c", "exit 100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(100))
	})

	It("podman exec in keep-id container drops privileges", func() {
		SkipIfNotRootless("This function is not enabled for rootful podman")
		ctrName := "testctr1"
		testCtr := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, "--userns=keep-id", ALPINE, "top"})
		testCtr.WaitWithDefaultTimeout()
		Expect(testCtr).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", ctrName, "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000000"))
	})

	It("podman exec --privileged", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		bndPerms := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--privileged", "--rm", ALPINE, "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		effPerms := session.OutputToString()

		setup := podmanTest.RunTopContainer("test-privileged")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "--privileged", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))

		session = podmanTest.Podman([]string{"exec", "--privileged", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))
	})

	It("podman exec --privileged", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "--user=bin", "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		bndPerms := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--privileged", "--user=bin", "--rm", ALPINE, "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		effPerms := session.OutputToString()

		setup := podmanTest.RunTopContainer("test-privileged")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=bin", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=bin", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))

	})

	It("podman exec --privileged", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		bndPerms := session.OutputToString()

		setup := podmanTest.RunTopContainer("test-privileged")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=bin", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("00000000"))

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=bin", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))
	})

	It("podman exec --privileged container not running as root", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		bndPerms := session.OutputToString()

		setup := podmanTest.RunTopContainerWithArgs("test-privileged", []string{"--user=bin"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "--privileged", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("00000000"))

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=bin", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("00000000"))

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=root", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))

		session = podmanTest.Podman([]string{"exec", "--privileged", "--user=bin", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))
	})

	It("podman exec with user with cap-add", func() {
		capAdd := "--cap-add=net_bind_service"
		session := podmanTest.Podman([]string{"run", "--user=bin", capAdd, "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		bndPerms := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--user=bin", capAdd, "--rm", ALPINE, "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		effPerms := session.OutputToString()

		setup := podmanTest.RunTopContainerWithArgs("test-privileged", []string{"--user=bin", capAdd})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))

		session = podmanTest.Podman([]string{"exec", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))
	})

	It("podman exec with user with and cap-drop cap-add", func() {
		capAdd := "--cap-add=net_bind_service"
		capDrop := "--cap-drop=all"
		session := podmanTest.Podman([]string{"run", "--user=bin", capDrop, capAdd, "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		bndPerms := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--user=bin", capDrop, capAdd, "--rm", ALPINE, "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		effPerms := session.OutputToString()

		setup := podmanTest.RunTopContainerWithArgs("test-privileged", []string{"--user=bin", capDrop, capAdd})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(bndPerms))

		session = podmanTest.Podman([]string{"exec", "--privileged", "test-privileged", "sh", "-c", "grep ^CapInh /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))

		session = podmanTest.Podman([]string{"exec", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))

		session = podmanTest.Podman([]string{"exec", "test-privileged", "sh", "-c", "grep ^CapPrm /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))

		session = podmanTest.Podman([]string{"exec", "test-privileged", "sh", "-c", "grep ^CapAmb /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(effPerms))
	})

	It("podman exec --privileged with user", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "--user=bin", "--rm", ALPINE, "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		bindPerms := session.OutputToString()

		setup := podmanTest.RunTopContainerWithArgs("test-privileged", []string{"--privileged", "--user=bin"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "--privileged", "test-privileged", "sh", "-c", "grep ^CapBnd /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(bindPerms))

		session = podmanTest.Podman([]string{"exec", "--privileged", "test-privileged", "sh", "-c", "grep ^CapEff /proc/self/status | cut -f 2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000000"))
	})

	It("podman exec terminal doesn't hang", func() {
		setup := podmanTest.Podman([]string{"run", "-dti", "--name", "test1", fedoraMinimal, "sleep", "+Inf"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		for i := 0; i < 5; i++ {
			session := podmanTest.Podman([]string{"exec", "-ti", "test1", "true"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}
	})

	It("podman exec pseudo-terminal sanity check", func() {
		setup := podmanTest.Podman([]string{"run", "--detach", "--name", "test1", fedoraMinimal, "sleep", "+Inf"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "--interactive", "--tty", "test1", "/usr/bin/stty", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(" onlcr"))
	})

	It("podman exec simple command with user", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "--user", "root", "test1", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman exec with user only in container", func() {
		testUser := "test123"
		setup := podmanTest.Podman([]string{"run", "--name", "test1", "-d", fedoraMinimal, "sleep", "60"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "test1", "useradd", testUser})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session2 := podmanTest.Podman([]string{"exec", "--user", testUser, "test1", "whoami"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
		Expect(session2.OutputToString()).To(Equal(testUser))
	})

	It("podman exec with user from run", func() {
		testUser := "guest"
		setup := podmanTest.Podman([]string{"run", "--user", testUser, "-d", ALPINE, "top"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))
		ctrID := setup.OutputToString()

		session := podmanTest.Podman([]string{"exec", ctrID, "whoami"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(testUser))

		overrideUser := "root"
		session = podmanTest.Podman([]string{"exec", "--user", overrideUser, ctrID, "whoami"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(overrideUser))
	})

	It("podman exec simple working directory test", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "--workdir", "/tmp", "test1", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("/tmp"))

		session = podmanTest.Podman([]string{"exec", "-w", "/tmp", "test1", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("/tmp"))
	})

	It("podman exec missing working directory test", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "--workdir", "/missing", "test1", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"exec", "-w", "/missing", "test1", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman exec cannot be invoked", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "test1", "/etc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(126))
	})

	It("podman exec command not found", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "test1", "notthere"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(127))
	})

	It("podman exec preserve fds sanity check", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		devNull, err := os.Open("/dev/null")
		Expect(err).To(BeNil())
		defer devNull.Close()
		files := []*os.File{
			devNull,
		}
		session := podmanTest.PodmanExtraFiles([]string{"exec", "--preserve-fds", "1", "test1", "ls"}, files)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman exec preserves --group-add groups", func() {
		groupName := "group1"
		gid := "4444"
		ctrName1 := "ctr1"
		ctr1 := podmanTest.Podman([]string{"run", "-ti", "--name", ctrName1, fedoraMinimal, "groupadd", "-g", gid, groupName})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(Exit(0))

		imgName := "img1"
		commit := podmanTest.Podman([]string{"commit", ctrName1, imgName})
		commit.WaitWithDefaultTimeout()
		Expect(commit).Should(Exit(0))

		ctrName2 := "ctr2"
		ctr2 := podmanTest.Podman([]string{"run", "-d", "--name", ctrName2, "--group-add", groupName, imgName, "sleep", "300"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(Exit(0))

		exec := podmanTest.Podman([]string{"exec", "-ti", ctrName2, "id"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))
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
		ctr := podmanTest.Podman([]string{"run", "-t", "-i", "-d", "--name", ctrName, "--user", "auser:first", "--group-add", "second", imgName, "sleep", "300"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		exec := podmanTest.Podman([]string{"exec", "-t", ctrName, "id"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))
		output := exec.OutputToString()
		Expect(output).To(ContainSubstring("4000(first)"))
		Expect(output).To(ContainSubstring("4001(second)"))
		Expect(output).To(ContainSubstring("1000(auser)"))

		// Kill the container just so the test does not take 15 seconds to stop.
		kill := podmanTest.Podman([]string{"kill", ctrName})
		kill.WaitWithDefaultTimeout()
		Expect(kill).Should(Exit(0))
	})

	It("podman exec --detach", func() {
		ctrName := "testctr"
		ctr := podmanTest.Podman([]string{"run", "-t", "-i", "-d", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		exec1 := podmanTest.Podman([]string{"exec", "-t", "-i", "-d", ctrName, "top"})
		exec1.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		data := podmanTest.InspectContainer(ctrName)
		Expect(data).To(HaveLen(1))
		Expect(data[0].ExecIDs).To(HaveLen(1))
		Expect(exec1.OutputToString()).To(ContainSubstring(data[0].ExecIDs[0]))

		exec2 := podmanTest.Podman([]string{"exec", "-t", "-i", ctrName, "ps", "-a"})
		exec2.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))
		Expect(strings.Count(exec2.OutputToString(), "top")).To(Equal(2))

		// Ensure that stop with a running detached exec session is
		// clean.
		stop := podmanTest.Podman([]string{"stop", ctrName})
		stop.WaitWithDefaultTimeout()
		Expect(stop).Should(Exit(0))
	})

	It("podman exec with env var secret", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "-t", "-i", "-d", "--secret", "source=mysecret,type=env", "--name", "secr", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "secr", "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(secretsString))

		session = podmanTest.Podman([]string{"commit", "secr", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "foobar.com/test1-image:latest", "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Not(ContainSubstring(secretsString)))
	})
})
