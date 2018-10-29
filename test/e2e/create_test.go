package integration

import (
	"fmt"
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman create", func() {
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

	It("podman create container based on a local image", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		cid := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		check := podmanTest.Podman([]string{"inspect", "-l"})
		check.WaitWithDefaultTimeout()
		data := check.InspectContainerToJSON()
		Expect(data[0].ID).To(ContainSubstring(cid))
	})

	It("podman create container based on a remote image", func() {
		session := podmanTest.Podman([]string{"create", BB_GLIBC, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman create using short options", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman create adds annotation", func() {
		session := podmanTest.Podman([]string{"create", "--annotation", "HELLO=WORLD", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		check := podmanTest.Podman([]string{"inspect", "-l"})
		check.WaitWithDefaultTimeout()
		data := check.InspectContainerToJSON()
		value, ok := data[0].Config.Annotations["HELLO"]
		Expect(ok).To(BeTrue())
		Expect(value).To(Equal("WORLD"))
	})

	It("podman create --entrypoint command", func() {
		session := podmanTest.Podman([]string{"create", "--entrypoint", "/bin/foobar", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"inspect", "-l", "--format", "{{.Config.Entrypoint}}"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("/bin/foobar"))
	})

	It("podman create --entrypoint \"\"", func() {
		session := podmanTest.Podman([]string{"create", "--entrypoint", "", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"inspect", "-l", "--format", "{{.Config.Entrypoint}}"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal(""))
	})

	It("podman create --entrypoint json", func() {
		jsonString := `[ "/bin/foo", "-c"]`
		session := podmanTest.Podman([]string{"create", "--entrypoint", jsonString, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"inspect", "-l", "--format", "{{.Config.Entrypoint}}"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("/bin/foo -c"))
	})
})
