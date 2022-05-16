package integration

import (
	"fmt"
	"os"
	"os/exec"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman system reset", func() {
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
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		_, _ = GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman system reset", func() {
		SkipIfRemote("system reset not supported on podman --remote")
		// system reset will not remove additional store images, so need to grab length

		// change the network dir so that we do not conflict with other tests
		// that would use the same network dir and cause unnecessary flakes
		podmanTest.NetworkConfigDir = tempdir

		session := podmanTest.Podman([]string{"rmi", "--force", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"images", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		l := len(session.OutputToStringArray())

		podmanTest.AddImageToRWStore(ALPINE)
		session = podmanTest.Podman([]string{"volume", "create", "data"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "-v", "data:/data", ALPINE, "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"system", "reset", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		Expect(session.ErrorToString()).To(Not(ContainSubstring("Failed to add pause process")))

		session = podmanTest.Podman([]string{"images", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(l))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())

		session = podmanTest.Podman([]string{"container", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())

		session = podmanTest.Podman([]string{"network", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// default network should exists
		Expect(session.OutputToStringArray()).To(HaveLen(1))

		// TODO: machine tests currently don't run outside of the machine test pkg
		// no machines are created here to cleanup
		session = podmanTest.Podman([]string{"machine", "list", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())
	})

	It("podman system reset, check podman.service", func() {
		SkipIfRemote("system reset not supported on podman --remote")
		SkipIfNotRootless("systemctl only works here with --user")
		SkipIfInContainer("systemd does not run in the containerized tests")

		sys, err := exec.LookPath("systemctl")
		if err != nil {
			Skip("systemctl not installed")
		}

		start := SystemExec(sys, []string{"--user", "start", "podman.service"})

		Expect(start.Exited).ShouldNot(Receive(), "Failed to start podman.service")

		// status should be running
		startStatus := SystemExec(sys, []string{"--user", "status", "podman.service"})

		Expect(startStatus.Exited).ShouldNot(Receive(), "Unit podman.service could not be found")
		Expect(startStatus.OutputToString()).To(ContainSubstring("active (running)"))

		sess := podmanTest.Podman([]string{"system", "reset", "-f"})
		sess.WaitWithDefaultTimeout()
		Expect(sess).Should(Exit(0))

		stopStatus := SystemExec(sys, []string{"--user", "status", "podman.service"})

		Expect(stopStatus.Exited).ShouldNot(Receive(), "Unit podman.service could not be found")
		Expect(stopStatus.OutputToString()).To(ContainSubstring("inactive (dead)"))
		if IsFedora() {
			Expect(stopStatus.OutputToString()).To(ContainSubstring("Invoking shutdown handler"))
		} else { // different outputs per OS
			Expect(stopStatus.OutputToString()).To(ContainSubstring("Stopping Podman API Service"))

		}

		start = StartSystemExec(sys, []string{"--user", "restart", "podman.service"})
		Expect(start.Exited).ShouldNot(Receive(), "Failed to start podman.service")
		// failure to restart leads to test flakes

	})
})
