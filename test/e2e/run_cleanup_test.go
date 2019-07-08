// +build !remoteclient

package integration

import (
	"os"
	"strings"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		result := podmanTest.Podman([]string{"run", "-dt", ALPINE, "top"})
		result.WaitWithDefaultTimeout()
		cid := result.OutputToString()
		Expect(result.ExitCode()).To(Equal(0))

		mount := SystemExec("mount", nil)
		Expect(mount.ExitCode()).To(Equal(0))
		Expect(strings.Contains(mount.OutputToString(), cid))

		pmount := podmanTest.Podman([]string{"mount", "--notruncate"})
		pmount.WaitWithDefaultTimeout()
		Expect(strings.Contains(pmount.OutputToString(), cid))
		Expect(pmount.ExitCode()).To(Equal(0))

		stop := podmanTest.Podman([]string{"stop", cid})
		stop.WaitWithDefaultTimeout()
		Expect(stop.ExitCode()).To(Equal(0))

		mount = SystemExec("mount", nil)
		Expect(mount.ExitCode()).To(Equal(0))
		Expect(!strings.Contains(mount.OutputToString(), cid))

		pmount = podmanTest.Podman([]string{"mount", "--notruncate"})
		pmount.WaitWithDefaultTimeout()
		Expect(!strings.Contains(pmount.OutputToString(), cid))
		Expect(pmount.ExitCode()).To(Equal(0))

	})
})
