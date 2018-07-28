package integration

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman tag", func() {
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

	It("podman tag shortname:latest", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		results := podmanTest.Podman([]string{"inspect", "foobar:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		inspectData := results.InspectImageJSON()
		Expect(StringInSlice("docker.io/library/alpine:latest", inspectData[0].RepoTags)).To(BeTrue())
		Expect(StringInSlice("localhost/foobar:latest", inspectData[0].RepoTags)).To(BeTrue())
	})

	It("podman tag shortname", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		results := podmanTest.Podman([]string{"inspect", "foobar:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		inspectData := results.InspectImageJSON()
		Expect(StringInSlice("docker.io/library/alpine:latest", inspectData[0].RepoTags)).To(BeTrue())
		Expect(StringInSlice("localhost/foobar:latest", inspectData[0].RepoTags)).To(BeTrue())
	})

	It("podman tag shortname:tag", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar:new"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		results := podmanTest.Podman([]string{"inspect", "foobar:new"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		inspectData := results.InspectImageJSON()
		Expect(StringInSlice("docker.io/library/alpine:latest", inspectData[0].RepoTags)).To(BeTrue())
		Expect(StringInSlice("localhost/foobar:new", inspectData[0].RepoTags)).To(BeTrue())
	})

	It("podman tag shortname image no tag", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		results := podmanTest.Podman([]string{"tag", "foobar", "barfoo"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))

		verify := podmanTest.Podman([]string{"inspect", "barfoo"})
		verify.WaitWithDefaultTimeout()
		Expect(verify.ExitCode()).To(Equal(0))
	})
})
