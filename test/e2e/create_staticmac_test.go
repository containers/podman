// +build !remoteclient

package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run with --mac-address flag", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootless()
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()
		// Cleanup the CNI networks used by the tests
		os.RemoveAll("/var/lib/cni/networks/podman")
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("Podman run --mac-address", func() {
		result := podmanTest.Podman([]string{"run", "--mac-address", "92:d0:c6:0a:29:34", ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(ContainSubstring("92:d0:c6:0a:29:34"))
	})
})
