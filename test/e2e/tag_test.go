package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman tag shortname:latest", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		results := podmanTest.Podman([]string{"inspect", "foobar:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(0))
		inspectData := results.InspectImageJSON()
		Expect(inspectData[0].RepoTags).To(ContainElement("quay.io/libpod/alpine:latest"))
		Expect(inspectData[0].RepoTags).To(ContainElement("localhost/foobar:latest"))
	})

	It("podman tag shortname", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		results := podmanTest.Podman([]string{"inspect", "foobar:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(0))
		inspectData := results.InspectImageJSON()
		Expect(inspectData[0].RepoTags).To(ContainElement("quay.io/libpod/alpine:latest"))
		Expect(inspectData[0].RepoTags).To(ContainElement("localhost/foobar:latest"))
	})

	It("podman tag shortname:tag", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar:new"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		results := podmanTest.Podman([]string{"inspect", "foobar:new"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(0))
		inspectData := results.InspectImageJSON()
		Expect(inspectData[0].RepoTags).To(ContainElement("quay.io/libpod/alpine:latest"))
		Expect(inspectData[0].RepoTags).To(ContainElement("localhost/foobar:new"))
	})

	It("podman tag shortname image no tag", func() {
		session := podmanTest.Podman([]string{"tag", ALPINE, "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		results := podmanTest.Podman([]string{"tag", "foobar", "barfoo"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(0))

		verify := podmanTest.Podman([]string{"inspect", "barfoo"})
		verify.WaitWithDefaultTimeout()
		Expect(verify).Should(Exit(0))
	})
})
