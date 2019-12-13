package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman tag shortname:latest", func() {
		session := podmanTest.PodmanNoCache([]string{"tag", ALPINE, "foobar:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		results := podmanTest.PodmanNoCache([]string{"inspect", "foobar:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		inspectData := results.InspectImageJSON()
		Expect(StringInSlice("docker.io/library/alpine:latest", inspectData[0].RepoTags)).To(BeTrue())
		Expect(StringInSlice("localhost/foobar:latest", inspectData[0].RepoTags)).To(BeTrue())
	})

	It("podman tag shortname", func() {
		session := podmanTest.PodmanNoCache([]string{"tag", ALPINE, "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		results := podmanTest.PodmanNoCache([]string{"inspect", "foobar:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		inspectData := results.InspectImageJSON()
		Expect(StringInSlice("docker.io/library/alpine:latest", inspectData[0].RepoTags)).To(BeTrue())
		Expect(StringInSlice("localhost/foobar:latest", inspectData[0].RepoTags)).To(BeTrue())
	})

	It("podman tag shortname:tag", func() {
		session := podmanTest.PodmanNoCache([]string{"tag", ALPINE, "foobar:new"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		results := podmanTest.PodmanNoCache([]string{"inspect", "foobar:new"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		inspectData := results.InspectImageJSON()
		Expect(StringInSlice("docker.io/library/alpine:latest", inspectData[0].RepoTags)).To(BeTrue())
		Expect(StringInSlice("localhost/foobar:new", inspectData[0].RepoTags)).To(BeTrue())
	})

	It("podman tag shortname image no tag", func() {
		session := podmanTest.PodmanNoCache([]string{"tag", ALPINE, "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		results := podmanTest.PodmanNoCache([]string{"tag", "foobar", "barfoo"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))

		verify := podmanTest.PodmanNoCache([]string{"inspect", "barfoo"})
		verify.WaitWithDefaultTimeout()
		Expect(verify.ExitCode()).To(Equal(0))
	})

	It("podman tag undo succeed", func() {
		const TAGGED_IMAGE = "foobar:latest"
		session := podmanTest.PodmanNoCache([]string{"tag", ALPINE, TAGGED_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		restore := podmanTest.PodmanNoCache([]string{"tag", TAGGED_IMAGE, "--restore"})
		restore.WaitWithDefaultTimeout()
		Expect(restore.ExitCode()).To(Equal(0))

		result := podmanTest.PodmanNoCache([]string{"images"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.LineInOutputContains(ALPINE)).To(BeTrue())
		Expect(result.LineInOutputContains(TAGGED_IMAGE)).To(BeFalse())
	})

	It("podman tag undo failed no history", func() {
		restore := podmanTest.PodmanNoCache([]string{"tag", ALPINE, "--restore"})
		restore.WaitWithDefaultTimeout()
		Expect(restore.ExitCode()).NotTo(Equal(0))
	})
})
