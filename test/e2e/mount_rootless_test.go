//go:build linux || freebsd

package integration

import (
	"slices"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman mount", func() {

	BeforeEach(func() {
		SkipIfNotRootless("This function is not enabled for rootful podman")
		SkipIfRemote("Podman mount not supported for remote connections")
	})

	It("podman mount", func() {
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cid := setup.OutputToString()

		mount := podmanTest.Podman([]string{"mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount).To(ExitWithError(125, "must execute `podman unshare` first"))
	})

	It("podman unshare podman mount", func() {
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cid := setup.OutputToString()

		// command: podman <options> unshare podman <options> mount cid
		args := []string{"unshare", podmanTest.PodmanBinary}
		opts := podmanTest.PodmanMakeOptions([]string{"mount", cid}, false, false)
		args = append(args, opts...)

		// container root file system location is podmanTest.TempDir/...
		// because "--root podmanTest.TempDir/..."
		session := podmanTest.Podman(args)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(podmanTest.TempDir))
	})

	It("podman image mount", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		mount := podmanTest.Podman([]string{"image", "mount", ALPINE})
		mount.WaitWithDefaultTimeout()
		Expect(mount).To(ExitWithError(125, "must execute `podman unshare` first"))
	})

	It("podman unshare image podman mount", func() {
		podmanTest.AddImageToRWStore(CITEST_IMAGE)

		// command: podman <options> unshare podman <options> image mount IMAGE
		args := []string{"unshare", podmanTest.PodmanBinary}
		opts := podmanTest.PodmanMakeOptions([]string{"image", "mount", CITEST_IMAGE}, false, false)
		args = append(args, opts...)

		// image location is podmanTest.TempDir/... because "--root podmanTest.TempDir/..."
		session := podmanTest.Podman(args)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(podmanTest.TempDir))

		// We have to unmount the image again otherwise we leak the tmpdir
		// as active mount points cannot be removed.
		index := slices.Index(args, "mount")
		Expect(index).To(BeNumerically(">", 0), "index should be found")
		args[index] = "unmount"
		session = podmanTest.Podman(args)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})
})
