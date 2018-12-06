package integration

import (
	"fmt"
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman run -d mount cleanup test", func() {
		mount := SystemExec("mount", nil)
		mount.WaitWithDefaultTimeout()
		out1 := mount.OutputToString()
		result := podmanTest.Podman([]string{"create", "-dt", ALPINE, "echo", "hello"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		mount = SystemExec("mount", nil)
		mount.WaitWithDefaultTimeout()
		out2 := mount.OutputToString()
		Expect(out1).To(Equal(out2))
	})
})
