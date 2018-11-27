package integration

import (
	"fmt"
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman create with --ip flag", func() {
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

	It("Podman create --ip with garbage address", func() {
		result := podmanTest.Podman([]string{"create", "--name", "test", "--ip", "114232346", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))
	})

	It("Podman create --ip with v6 address", func() {
		result := podmanTest.Podman([]string{"create", "--name", "test", "--ip", "2001:db8:bad:beef::1", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))
	})

	It("Podman create --ip with non-allocatable IP", func() {
		result := podmanTest.Podman([]string{"create", "--name", "test", "--ip", "203.0.113.124", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"start", "test"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))
	})

	It("Podman create with specified static IP has correct IP", func() {
		result := podmanTest.Podman([]string{"create", "--name", "test", "--ip", "10.88.64.128", ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"start", "test"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"logs", "test"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(ContainSubstring("10.88.64.128/16"))
	})

	It("Podman create two containers with the same IP", func() {
		result := podmanTest.Podman([]string{"create", "--name", "test1", "--ip", "10.88.64.128", ALPINE, "sleep", "999"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		result = podmanTest.Podman([]string{"create", "--name", "test2", "--ip", "10.88.64.128", ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		result = podmanTest.Podman([]string{"start", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		result = podmanTest.Podman([]string{"start", "test2"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))
	})
})
