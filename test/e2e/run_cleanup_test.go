package integration

import (
	"os"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run exit", func() {
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
		podmanTest.RestoreArtifact(ALPINE)
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman run -d mount cleanup test", func() {
		SkipIfRemote("podman-remote does not support mount")
		SkipIfRootless("FIXME podman mount requires podman unshare first")

		result := podmanTest.Podman([]string{"run", "-dt", ALPINE, "top"})
		result.WaitWithDefaultTimeout()
		cid := result.OutputToString()
		Expect(result).Should(Exit(0))

		mount := SystemExec("mount", nil)
		Expect(mount).Should(Exit(0))
		Expect(mount.OutputToString()).To(ContainSubstring(cid))

		pmount := podmanTest.Podman([]string{"mount", "--no-trunc"})
		pmount.WaitWithDefaultTimeout()
		Expect(pmount).Should(Exit(0))
		Expect(pmount.OutputToString()).To(ContainSubstring(cid))

		stop := podmanTest.Podman([]string{"stop", cid})
		stop.WaitWithDefaultTimeout()
		Expect(stop).Should(Exit(0))

		// We have to force cleanup so the unmount happens
		podmanCleanupSession := podmanTest.Podman([]string{"container", "cleanup", cid})
		podmanCleanupSession.WaitWithDefaultTimeout()
		Expect(podmanCleanupSession).Should(Exit(0))

		mount = SystemExec("mount", nil)
		Expect(mount).Should(Exit(0))
		Expect(mount.OutputToString()).NotTo(ContainSubstring(cid))

		pmount = podmanTest.Podman([]string{"mount", "--no-trunc"})
		pmount.WaitWithDefaultTimeout()
		Expect(pmount).Should(Exit(0))
		Expect(pmount.OutputToString()).NotTo(ContainSubstring(cid))

	})
})
