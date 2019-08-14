package integration

import (
	"os"

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
		Expect(session.ExitCode()).To(Not(Equal(0)))

		session = podmanTest.Podman([]string{"exec", "-w", "/missing", "test1", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
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
})
