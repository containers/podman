package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
)

var _ = Describe("Podman inspect", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()

	})

	It("podman inspect alpine image", func() {
		session := podmanTest.Podman([]string{"inspect", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
		imageData := session.InspectImageJSON()
		Expect(imageData.RepoTags[0]).To(Equal("docker.io/library/alpine:latest"))
	})

	It("podman inspect bogus container", func() {
		session := podmanTest.Podman([]string{"inspect", "foobar4321"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman inspect with GO format", func() {
		session := podmanTest.Podman([]string{"inspect", "--format", "{{.ID}}", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"images", "-q", "--no-trunc", ALPINE})
		result.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(strings.Trim(result.OutputToString(), "sha256:")).To(Equal(session.OutputToString()))
	})

	It("podman inspect specified type", func() {
		session := podmanTest.Podman([]string{"inspect", "--type", "image", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman inspect container with size", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"inspect", "--size", "-l"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		conData := result.InspectContainerToJSON()
		Expect(conData.CtrInspectData.SizeRootFs).To(BeNumerically(">", 0))
	})
})
