// +build !remoteclient

package integration

import (
	"os"

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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman run -d mount cleanup test", func() {
		mount := SystemExec("mount", nil)
		Expect(mount.ExitCode()).To(Equal(0))

		out1 := mount.OutputToString()
		result := podmanTest.Podman([]string{"create", "-dt", ALPINE, "echo", "hello"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		mount = SystemExec("mount", nil)
		Expect(mount.ExitCode()).To(Equal(0))

		out2 := mount.OutputToString()
		Expect(out1).To(Equal(out2))
	})
})
