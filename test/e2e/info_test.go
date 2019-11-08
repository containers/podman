// +build !remoteclient

package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman Info", func() {
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
		processTestResult(f)

	})

	It("podman info json output", func() {
		session := podmanTest.Podman([]string{"info", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

	})
	It("podman system info json output", func() {
		session := podmanTest.Podman([]string{"system", "info", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

	})
	It("podman info --format GO template", func() {
		session := podmanTest.Podman([]string{"info", "--format", "{{ json .}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})
})
