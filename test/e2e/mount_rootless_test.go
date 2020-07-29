// +build !remote

package integration

import (
	"os"

	. "github.com/containers/podman/v2/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman mount", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		if os.Geteuid() == 0 {
			Skip("This function is not enabled for rootfull podman")
		}
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
		Expect(setup.ExitCode()).To(Equal(0))
		cid := setup.OutputToString()

		mount := podmanTest.Podman([]string{"mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount.ExitCode()).ToNot(Equal(0))
		Expect(mount.ErrorToString()).To(ContainSubstring("podman unshare"))
	})

	It("podman unshare podman mount", func() {
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid := setup.OutputToString()

		session := podmanTest.Podman([]string{"unshare", PODMAN_BINARY, "mount", cid})
		session.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
	})

	It("podman image mount", func() {
		setup := podmanTest.PodmanNoCache([]string{"pull", ALPINE})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		mount := podmanTest.PodmanNoCache([]string{"image", "mount", ALPINE})
		mount.WaitWithDefaultTimeout()
		Expect(mount.ExitCode()).ToNot(Equal(0))
		Expect(mount.ErrorToString()).To(ContainSubstring("podman unshare"))
	})

	It("podman unshare image podman mount", func() {
		setup := podmanTest.PodmanNoCache([]string{"pull", ALPINE})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"unshare", PODMAN_BINARY, "image", "mount", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
	})
})
