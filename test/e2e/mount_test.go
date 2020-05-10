// +build !remoteclient

package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
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
		SkipIfRootlessV2()
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
		Expect(mount.ExitCode()).To(Equal(0))

		umount := podmanTest.Podman([]string{"umount", cid})
		umount.WaitWithDefaultTimeout()
		Expect(umount.ExitCode()).To(Equal(0))
	})

	It("podman container mount", func() {
		setup := podmanTest.Podman([]string{"container", "create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid := setup.OutputToString()

		mount := podmanTest.Podman([]string{"container", "mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount.ExitCode()).To(Equal(0))

		umount := podmanTest.Podman([]string{"container", "umount", cid})
		umount.WaitWithDefaultTimeout()
		Expect(umount.ExitCode()).To(Equal(0))
	})

	It("podman mount with json format", func() {
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid := setup.OutputToString()

		mount := podmanTest.Podman([]string{"mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount.ExitCode()).To(Equal(0))

		j := podmanTest.Podman([]string{"mount", "--format=json"})
		j.WaitWithDefaultTimeout()
		Expect(j.ExitCode()).To(Equal(0))
		Expect(j.IsJSONOutputValid()).To(BeTrue())

		umount := podmanTest.Podman([]string{"umount", cid})
		umount.WaitWithDefaultTimeout()
		Expect(umount.ExitCode()).To(Equal(0))
	})

	It("podman mount many", func() {
		setup1 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup1.WaitWithDefaultTimeout()
		Expect(setup1.ExitCode()).To(Equal(0))
		cid1 := setup1.OutputToString()

		setup2 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup2.WaitWithDefaultTimeout()
		Expect(setup2.ExitCode()).To(Equal(0))
		cid2 := setup2.OutputToString()

		setup3 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup3.WaitWithDefaultTimeout()
		Expect(setup3.ExitCode()).To(Equal(0))
		cid3 := setup3.OutputToString()

		mount1 := podmanTest.Podman([]string{"mount", cid1, cid2, cid3})
		mount1.WaitWithDefaultTimeout()
		Expect(mount1.ExitCode()).To(Equal(0))

		umount := podmanTest.Podman([]string{"umount", cid1, cid2, cid3})
		umount.WaitWithDefaultTimeout()
		Expect(umount.ExitCode()).To(Equal(0))
	})

	It("podman umount many", func() {
		setup1 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup1.WaitWithDefaultTimeout()
		Expect(setup1.ExitCode()).To(Equal(0))
		cid1 := setup1.OutputToString()

		setup2 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup2.WaitWithDefaultTimeout()
		Expect(setup2.ExitCode()).To(Equal(0))
		cid2 := setup2.OutputToString()

		mount1 := podmanTest.Podman([]string{"mount", cid1})
		mount1.WaitWithDefaultTimeout()
		Expect(mount1.ExitCode()).To(Equal(0))

		mount2 := podmanTest.Podman([]string{"mount", cid2})
		mount2.WaitWithDefaultTimeout()
		Expect(mount2.ExitCode()).To(Equal(0))

		umount := podmanTest.Podman([]string{"umount", cid1, cid2})
		umount.WaitWithDefaultTimeout()
		Expect(umount.ExitCode()).To(Equal(0))
	})

	It("podman umount all", func() {
		setup1 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup1.WaitWithDefaultTimeout()
		Expect(setup1.ExitCode()).To(Equal(0))
		cid1 := setup1.OutputToString()

		setup2 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup2.WaitWithDefaultTimeout()
		Expect(setup2.ExitCode()).To(Equal(0))
		cid2 := setup2.OutputToString()

		mount1 := podmanTest.Podman([]string{"mount", cid1})
		mount1.WaitWithDefaultTimeout()
		Expect(mount1.ExitCode()).To(Equal(0))

		mount2 := podmanTest.Podman([]string{"mount", cid2})
		mount2.WaitWithDefaultTimeout()
		Expect(mount2.ExitCode()).To(Equal(0))

		umount := podmanTest.Podman([]string{"umount", "--all"})
		umount.WaitWithDefaultTimeout()
		Expect(umount.ExitCode()).To(Equal(0))
	})

	It("podman list mounted container", func() {
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid := setup.OutputToString()

		lmount := podmanTest.Podman([]string{"mount", "--notruncate"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount.ExitCode()).To(Equal(0))
		Expect(lmount.OutputToString()).To(Equal(""))

		mount := podmanTest.Podman([]string{"mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount.ExitCode()).To(Equal(0))

		lmount = podmanTest.Podman([]string{"mount", "--notruncate"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount.ExitCode()).To(Equal(0))
		Expect(lmount.OutputToString()).To(ContainSubstring(cid))

		umount := podmanTest.Podman([]string{"umount", cid})
		umount.WaitWithDefaultTimeout()
		Expect(umount.ExitCode()).To(Equal(0))
	})

	It("podman list running container", func() {
		SkipIfRootless()

		setup := podmanTest.Podman([]string{"run", "-dt", ALPINE, "top"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid := setup.OutputToString()

		lmount := podmanTest.Podman([]string{"mount", "--notruncate"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount.ExitCode()).To(Equal(0))
		Expect(lmount.OutputToString()).To(ContainSubstring(cid))

		stop := podmanTest.Podman([]string{"stop", cid})
		stop.WaitWithDefaultTimeout()
		Expect(stop.ExitCode()).To(Equal(0))

		lmount = podmanTest.Podman([]string{"mount", "--notruncate"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount.ExitCode()).To(Equal(0))
		Expect(lmount.OutputToString()).To(Equal(""))
	})

	It("podman list multiple mounted containers", func() {
		SkipIfRootless()

		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid1 := setup.OutputToString()

		setup = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid2 := setup.OutputToString()

		setup = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid3 := setup.OutputToString()

		lmount := podmanTest.Podman([]string{"mount", "--notruncate"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount.ExitCode()).To(Equal(0))
		Expect(lmount.OutputToString()).To(Equal(""))

		mount := podmanTest.Podman([]string{"mount", cid1, cid3})
		mount.WaitWithDefaultTimeout()
		Expect(mount.ExitCode()).To(Equal(0))

		lmount = podmanTest.Podman([]string{"mount", "--notruncate"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount.ExitCode()).To(Equal(0))
		Expect(lmount.OutputToString()).To(ContainSubstring(cid1))
		Expect(lmount.OutputToString()).ToNot(ContainSubstring(cid2))
		Expect(lmount.OutputToString()).To(ContainSubstring(cid3))

		umount := podmanTest.Podman([]string{"umount", cid1, cid3})
		umount.WaitWithDefaultTimeout()
		Expect(umount.ExitCode()).To(Equal(0))

		lmount = podmanTest.Podman([]string{"mount", "--notruncate"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount.ExitCode()).To(Equal(0))
		Expect(lmount.OutputToString()).To(Equal(""))

	})

	It("podman list mounted container", func() {
		SkipIfRootless()

		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid := setup.OutputToString()

		lmount := podmanTest.Podman([]string{"mount", "--notruncate"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount.ExitCode()).To(Equal(0))
		Expect(lmount.OutputToString()).To(Equal(""))

		mount := podmanTest.Podman([]string{"mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount.ExitCode()).To(Equal(0))

		lmount = podmanTest.Podman([]string{"mount", "--notruncate"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount.ExitCode()).To(Equal(0))
		Expect(lmount.OutputToString()).To(ContainSubstring(cid))

		umount := podmanTest.Podman([]string{"umount", cid})
		umount.WaitWithDefaultTimeout()
		Expect(umount.ExitCode()).To(Equal(0))
	})
})
