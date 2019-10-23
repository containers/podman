package integration

import (
	"fmt"
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman volume create", func() {
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
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.CleanupVolume()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman create volume", func() {
		session := podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"volume", "ls", "-q"})
		check.WaitWithDefaultTimeout()
		match, _ := check.GrepString(volName)
		Expect(match).To(BeTrue())
		Expect(len(check.OutputToStringArray())).To(Equal(1))
	})

	It("podman create volume with name", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"volume", "ls", "-q"})
		check.WaitWithDefaultTimeout()
		match, _ := check.GrepString(volName)
		Expect(match).To(BeTrue())
		Expect(len(check.OutputToStringArray())).To(Equal(1))
	})

	It("podman create volume with bad volume option", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--opt", "badOpt=bad"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman create volume with o=uid,gid", func() {
		volName := "testVol"
		uid := "3000"
		gid := "4000"
		session := podmanTest.Podman([]string{"volume", "create", "--opt", fmt.Sprintf("o=uid=%s,gid=%s", uid, gid), volName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		inspectUID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .UID }}", volName})
		inspectUID.WaitWithDefaultTimeout()
		Expect(inspectUID.ExitCode()).To(Equal(0))
		Expect(inspectUID.OutputToString()).To(Equal(uid))

		inspectGID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .GID }}", volName})
		inspectGID.WaitWithDefaultTimeout()
		Expect(inspectGID.ExitCode()).To(Equal(0))
		Expect(inspectGID.OutputToString()).To(Equal(gid))
	})
})
