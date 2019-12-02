package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	"github.com/containers/libpod/version"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman version", func() {
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
		podmanTest.SeedImages()

	})

	It("podman version", func() {
		session := podmanTest.Podman([]string{"version"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 2))
		ok, _ := session.GrepString(version.Version)
		Expect(ok).To(BeTrue())
	})

	It("podman -v", func() {
		session := podmanTest.Podman([]string{"-v"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString(version.Version)
		Expect(ok).To(BeTrue())
	})

	It("podman --version", func() {
		session := podmanTest.Podman([]string{"--version"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString(version.Version)
		Expect(ok).To(BeTrue())
	})

	It("podman version --format json", func() {
		session := podmanTest.Podman([]string{"version", "--format", "json"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman version --format json", func() {
		session := podmanTest.Podman([]string{"version", "--format", "{{ json .}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman version --format GO template", func() {
		session := podmanTest.Podman([]string{"version", "--format", "{{ .Client.Version }}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"version", "--format", "{{ .Server.Version }}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"version", "--format", "{{ .Version }}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
})
