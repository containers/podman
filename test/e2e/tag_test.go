package integration

import (
	"os"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman tag", func() {
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
		podmanTest.AddImageToRWStore(ALPINE)
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman tag shortname:latest", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		results := podmanTest.Podman([]string{"inspect", "foobar:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		inspectData := results.InspectImageJSON()
		Expect(StringInSlice("quay.io/libpod/alpine:latest", inspectData[0].RepoTags)).To(BeTrue())
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
		Expect(StringInSlice("quay.io/libpod/alpine:latest", inspectData[0].RepoTags)).To(BeTrue())
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
		Expect(StringInSlice("quay.io/libpod/alpine:latest", inspectData[0].RepoTags)).To(BeTrue())
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
