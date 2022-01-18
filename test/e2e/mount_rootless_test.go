package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman mount", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfNotRootless("This function is not enabled for rootful podman")
		SkipIfRemote("Podman mount not supported for remote connections")
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman mount", func() {
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))
		cid := setup.OutputToString()

		mount := podmanTest.Podman([]string{"mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount).To(ExitWithError())
		Expect(mount.ErrorToString()).To(ContainSubstring("podman unshare"))
	})

	It("podman unshare podman mount", func() {
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))
		cid := setup.OutputToString()

		session := podmanTest.Podman([]string{"unshare", PODMAN_BINARY, "mount", cid})
		session.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))
	})

	It("podman image mount", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		mount := podmanTest.Podman([]string{"image", "mount", ALPINE})
		mount.WaitWithDefaultTimeout()
		Expect(mount).To(ExitWithError())
		Expect(mount.ErrorToString()).To(ContainSubstring("podman unshare"))
	})

	It("podman unshare image podman mount", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		setup := podmanTest.Podman([]string{"pull", ALPINE})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"unshare", PODMAN_BINARY, "image", "mount", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))
	})
})
