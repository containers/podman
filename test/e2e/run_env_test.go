package integration

import (
	"os"

	. "github.com/containers/podman/v3/test/utils"
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

	It("podman run --env-host environment test", func() {
		env := append(os.Environ(), "FOO=BAR")
		session := podmanTest.PodmanAsUser([]string{"run", "--rm", "--env-host", ALPINE, "/bin/printenv", "FOO"}, 0, 0, "", env)
		session.WaitWithDefaultTimeout()
		if IsRemote() {
			// podman-remote does not support --env-host
			Expect(session.ExitCode()).To(Equal(125))
			Expect(session.ErrorToString()).To(ContainSubstring("unknown flag: --env-host"))
			return
		}
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("BAR")
		Expect(match).Should(BeTrue())

		session = podmanTest.PodmanAsUser([]string{"run", "--rm", "--env", "FOO=BAR1", "--env-host", ALPINE, "/bin/printenv", "FOO"}, 0, 0, "", env)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ = session.GrepString("BAR1")
		Expect(match).Should(BeTrue())
		os.Unsetenv("FOO")
	})

	It("podman run --http-proxy test", func() {
		os.Setenv("http_proxy", "1.2.3.4")
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		}
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "printenv", "http_proxy"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("1.2.3.4")
		Expect(match).Should(BeTrue())

		session = podmanTest.Podman([]string{"run", "--http-proxy=false", ALPINE, "printenv", "http_proxy"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(1))
		Expect(session.OutputToString()).To(Equal(""))

		session = podmanTest.Podman([]string{"run", "--env", "http_proxy=5.6.7.8", ALPINE, "printenv", "http_proxy"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ = session.GrepString("5.6.7.8")
		Expect(match).Should(BeTrue())
		os.Unsetenv("http_proxy")

		session = podmanTest.Podman([]string{"run", "--http-proxy=false", "--env", "http_proxy=5.6.7.8", ALPINE, "printenv", "http_proxy"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ = session.GrepString("5.6.7.8")
		Expect(match).Should(BeTrue())
		os.Unsetenv("http_proxy")
	})
})
