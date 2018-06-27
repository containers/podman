package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman mount", func() {
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
		Expect(j.IsJSONOutputValid())

		umount := podmanTest.Podman([]string{"umount", cid})
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
})
