package integration

import (
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman inspect alpine image", func() {
		session := podmanTest.Podman([]string{"inspect", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
		imageData := session.InspectImageJSON()
		Expect(imageData[0].RepoTags[0]).To(Equal("docker.io/library/alpine:latest"))
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
		Expect(conData[0].SizeRootFs).To(BeNumerically(">", 0))
	})

	It("podman inspect container and image", func() {
		ls, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		cid := ls.OutputToString()

		result := podmanTest.Podman([]string{"inspect", "--format={{.ID}}", cid, ALPINE})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(Equal(2))
	})

	It("podman inspect -l with additional input should fail", func() {
		result := podmanTest.Podman([]string{"inspect", "-l", "1234foobar"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
	})

})
