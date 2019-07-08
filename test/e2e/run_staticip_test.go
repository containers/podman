// +build !remoteclient

package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run with --ip flag", func() {
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

	It("Podman run --ip with garbage address", func() {
		result := podmanTest.Podman([]string{"run", "-ti", "--ip", "114232346", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))
	})

	It("Podman run --ip with v6 address", func() {
		result := podmanTest.Podman([]string{"run", "-ti", "--ip", "2001:db8:bad:beef::1", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))
	})

	It("Podman run --ip with non-allocatable IP", func() {
		result := podmanTest.Podman([]string{"run", "-ti", "--ip", "203.0.113.124", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))
	})

	It("Podman run with specified static IP has correct IP", func() {
		result := podmanTest.Podman([]string{"run", "-ti", "--ip", "10.88.63.2", ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(ContainSubstring("10.88.63.2/16"))
	})

	It("Podman run two containers with the same IP", func() {
		result := podmanTest.Podman([]string{"run", "-d", "--ip", "10.88.64.128", ALPINE, "sleep", "999"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		result = podmanTest.Podman([]string{"run", "-ti", "--ip", "10.88.64.128", ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))
	})
})
