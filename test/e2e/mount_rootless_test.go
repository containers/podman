package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
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

		// command: podman <options> unshare podman <options> mount cid
		args := []string{"unshare", podmanTest.PodmanBinary}
		opts := podmanTest.PodmanMakeOptions([]string{"mount", cid}, false, false)
		args = append(args, opts...)

		// container root file system location is podmanTest.TempDir/...
		// because "--root podmanTest.TempDir/..."
		session := podmanTest.Podman(args)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(podmanTest.TempDir))
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

		// command: podman <options> unshare podman <options> image mount ALPINE
		args := []string{"unshare", podmanTest.PodmanBinary}
		opts := podmanTest.PodmanMakeOptions([]string{"image", "mount", ALPINE}, false, false)
		args = append(args, opts...)

		// image location is podmanTest.TempDir/... because "--root podmanTest.TempDir/..."
		session := podmanTest.Podman(args)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(podmanTest.TempDir))
	})
})
