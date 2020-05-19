package integration

import (
	"fmt"
	"os"
	"strings"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman exec", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		Skip(v2remotefail)
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
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman exec without command", func() {
		session := podmanTest.Podman([]string{"exec", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman exec simple command", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "test1", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman container exec simple command", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"container", "exec", "test1", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman exec simple command using latest", func() {
		// the remote client doesn't use latest
		SkipIfRemote()
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "-l", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman exec environment test", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "--env", "FOO=BAR", "test1", "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("BAR")
		Expect(match).Should(BeTrue())

		session = podmanTest.Podman([]string{"exec", "--env", "PATH=/bin", "test1", "printenv", "PATH"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ = session.GrepString("/bin")
		Expect(match).Should(BeTrue())
	})

	It("podman exec os.Setenv env", func() {
		// remote doesn't properly interpret os.Setenv
		SkipIfRemote()
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		os.Setenv("FOO", "BAR")
		session := podmanTest.Podman([]string{"exec", "--env", "FOO", "test1", "printenv", "FOO"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("BAR")
		Expect(match).Should(BeTrue())
		os.Unsetenv("FOO")
	})

	It("podman exec exit code", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "test1", "sh", "-c", "exit 100"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(100))
	})

	It("podman exec terminal doesn't hang", func() {
		setup := podmanTest.Podman([]string{"run", "-dti", fedoraMinimal, "sleep", "+Inf"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		for i := 0; i < 5; i++ {
			session := podmanTest.Podman([]string{"exec", "-lti", "true"})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))
		}
	})

	It("podman exec pseudo-terminal sanity check", func() {
		setup := podmanTest.Podman([]string{"run", "--detach", "--name", "test1", fedoraMinimal, "sleep", "+Inf"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "--interactive", "--tty", "test1", "/usr/bin/stty", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString(" onlcr")
		Expect(match).Should(BeTrue())
	})

	It("podman exec simple command with user", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "--user", "root", "test1", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman exec with user only in container", func() {
		testUser := "test123"
		setup := podmanTest.Podman([]string{"run", "--name", "test1", "-d", fedoraMinimal, "sleep", "60"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "test1", "useradd", testUser})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"exec", "--user", testUser, "test1", "whoami"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))
		Expect(session2.OutputToString()).To(Equal(testUser))
	})

	It("podman exec with user from run", func() {
		testUser := "guest"
		setup := podmanTest.Podman([]string{"run", "--user", testUser, "-d", ALPINE, "top"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		ctrID := setup.OutputToString()

		session := podmanTest.Podman([]string{"exec", ctrID, "whoami"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring(testUser))

		overrideUser := "root"
		session = podmanTest.Podman([]string{"exec", "--user", overrideUser, ctrID, "whoami"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring(overrideUser))
	})

	It("podman exec simple working directory test", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "--workdir", "/tmp", "test1", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("/tmp")
		Expect(match).Should(BeTrue())

		session = podmanTest.Podman([]string{"exec", "-w", "/tmp", "test1", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ = session.GrepString("/tmp")
		Expect(match).Should(BeTrue())
	})

	It("podman exec missing working directory test", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

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
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "test1", "/etc"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(126))
	})

	It("podman exec command not found", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "test1", "notthere"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(127))
	})

	It("podman exec preserve fds sanity check", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		devNull, err := os.Open("/dev/null")
		Expect(err).To(BeNil())
		defer devNull.Close()
		files := []*os.File{
			devNull,
		}
		session := podmanTest.PodmanExtraFiles([]string{"exec", "--preserve-fds", "1", "test1", "ls"}, files)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman exec preserves --group-add groups", func() {
		groupName := "group1"
		gid := "4444"
		ctrName1 := "ctr1"
		ctr1 := podmanTest.Podman([]string{"run", "-ti", "--name", ctrName1, fedoraMinimal, "groupadd", "-g", gid, groupName})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1.ExitCode()).To(Equal(0))

		imgName := "img1"
		commit := podmanTest.Podman([]string{"commit", ctrName1, imgName})
		commit.WaitWithDefaultTimeout()
		Expect(commit.ExitCode()).To(Equal(0))

		ctrName2 := "ctr2"
		ctr2 := podmanTest.Podman([]string{"run", "-d", "--name", ctrName2, "--group-add", groupName, imgName, "sleep", "300"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2.ExitCode()).To(Equal(0))

		exec := podmanTest.Podman([]string{"exec", "-ti", ctrName2, "id"})
		exec.WaitWithDefaultTimeout()
		Expect(exec.ExitCode()).To(Equal(0))
		Expect(strings.Contains(exec.OutputToString(), fmt.Sprintf("%s(%s)", gid, groupName))).To(BeTrue())
	})
})
