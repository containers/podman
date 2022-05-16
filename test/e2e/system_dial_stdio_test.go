package integration

import (
	"fmt"
	"os"
	"os/exec"

	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/podman/v4/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman system dial-stdio", func() {
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

	It("podman system dial-stdio help", func() {
		session := podmanTest.Podman([]string{"system", "dial-stdio", "--help"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("Examples: podman system dial-stdio"))
	})

	It("podman system dial-stdio while service is not running", func() {
		if IsRemote() {
			Skip("this test is only for non-remote")
		}
		SkipIfInContainer("systemd does not run in the containerized tests")

		// due to other tests modifying the service, we need to stop it here.
		sys, err := exec.LookPath("systemctl")
		if err != nil || !utils.RunsOnSystemd() {
			Skip("systemctl not installed")
		}

		args := []string{}
		if isRootless() {
			args = append(args, "--user")
		}
		args = append(args, "stop", "podman")
		stop := StartSystemExec(sys, args)
		stop.WaitWithDefaultTimeout()
		Expect(stop.Exited).ShouldNot(Receive(), "Failed to stop podman.service")

		session := podmanTest.Podman([]string{"system", "dial-stdio"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("Error: failed to open connection to podman"))
	})
})
