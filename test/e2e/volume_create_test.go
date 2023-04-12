package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
	})

	AfterEach(func() {
		podmanTest.CleanupVolume()
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman create volume", func() {
		session := podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"volume", "ls", "-q"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToString()).To(ContainSubstring(volName))
		Expect(check.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman create volume with name", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"volume", "ls", "-q"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToString()).To(ContainSubstring(volName))
		Expect(check.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman create volume with existing name fails", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman create volume --ignore", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "create", "--ignore", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(volName))
	})

	It("podman create and export volume", func() {
		if podmanTest.RemoteTest {
			Skip("Volume export check does not work with a remote client")
		}

		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data", ALPINE, "sh", "-c", "echo hello >> " + "/data/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"volume", "export", volName})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman create and import volume", func() {
		if podmanTest.RemoteTest {
			Skip("Volume export check does not work with a remote client")
		}

		session := podmanTest.Podman([]string{"volume", "create", "my_vol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data", ALPINE, "sh", "-c", "echo hello >> " + "/data/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		helloTar := filepath.Join(podmanTest.TempDir, "hello.tar")
		session = podmanTest.Podman([]string{"volume", "export", volName, "--output", helloTar})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "create", "my_vol2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "import", "my_vol2", helloTar})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(""), "output of volume import")

		session = podmanTest.Podman([]string{"run", "--volume", "my_vol2:/data", ALPINE, "cat", "/data/test"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman import/export volume should fail", func() {
		// try import on volume or source which does not exist
		SkipIfRemote("Volume export check does not work with a remote client")

		session := podmanTest.Podman([]string{"volume", "import", "notfound", "notfound.tar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("open notfound.tar: no such file or directory"))

		session = podmanTest.Podman([]string{"volume", "import", "notfound", "-"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("no such volume notfound"))

		session = podmanTest.Podman([]string{"volume", "export", "notfound"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("no such volume notfound"))
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
		Expect(session).Should(Exit(0))

		inspectUID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .UID }}", volName})
		inspectUID.WaitWithDefaultTimeout()
		Expect(inspectUID).Should(Exit(0))
		Expect(inspectUID.OutputToString()).To(Equal(uid))

		inspectGID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .GID }}", volName})
		inspectGID.WaitWithDefaultTimeout()
		Expect(inspectGID).Should(Exit(0))
		Expect(inspectGID.OutputToString()).To(Equal(gid))

		// options should contain `uid=3000,gid=4000:3000:4000`
		optionFormat := `{{ .Options.o }}:{{ .Options.UID }}:{{ .Options.GID }}`
		optionStrFormatExpect := fmt.Sprintf(`uid=%s,gid=%s:%s:%s`, uid, gid, uid, gid)
		inspectOpts := podmanTest.Podman([]string{"volume", "inspect", "--format", optionFormat, volName})
		inspectOpts.WaitWithDefaultTimeout()
		Expect(inspectOpts).Should(Exit(0))
		Expect(inspectOpts.OutputToString()).To(Equal(optionStrFormatExpect))
	})

	It("image-backed volume basic functionality", func() {
		podmanTest.AddImageToRWStore(fedoraMinimal)
		volName := "testvol"
		volCreate := podmanTest.Podman([]string{"volume", "create", "--driver", "image", "--opt", fmt.Sprintf("image=%s", fedoraMinimal), volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(Exit(0))

		runCmd := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "cat", "/test/etc/redhat-release"})
		runCmd.WaitWithDefaultTimeout()
		Expect(runCmd).Should(Exit(0))
		Expect(runCmd.OutputToString()).To(ContainSubstring("Fedora"))

		rmCmd := podmanTest.Podman([]string{"rmi", "--force", fedoraMinimal})
		rmCmd.WaitWithDefaultTimeout()
		Expect(rmCmd).Should(Exit(0))

		psCmd := podmanTest.Podman([]string{"ps", "-aq"})
		psCmd.WaitWithDefaultTimeout()
		Expect(psCmd).Should(Exit(0))
		Expect(psCmd.OutputToString()).To(BeEmpty())

		volumesCmd := podmanTest.Podman([]string{"volume", "ls", "-q"})
		volumesCmd.WaitWithDefaultTimeout()
		Expect(volumesCmd).Should(Exit(0))
		Expect(volumesCmd.OutputToString()).To(Not(ContainSubstring(volName)))
	})

	It("image-backed volume force removal", func() {
		podmanTest.AddImageToRWStore(fedoraMinimal)
		volName := "testvol"
		volCreate := podmanTest.Podman([]string{"volume", "create", "--driver", "image", "--opt", fmt.Sprintf("image=%s", fedoraMinimal), volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(Exit(0))

		runCmd := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "cat", "/test/etc/redhat-release"})
		runCmd.WaitWithDefaultTimeout()
		Expect(runCmd).Should(Exit(0))
		Expect(runCmd.OutputToString()).To(ContainSubstring("Fedora"))

		rmCmd := podmanTest.Podman([]string{"volume", "rm", "--force", volName})
		rmCmd.WaitWithDefaultTimeout()
		Expect(rmCmd).Should(Exit(0))

		psCmd := podmanTest.Podman([]string{"ps", "-aq"})
		psCmd.WaitWithDefaultTimeout()
		Expect(psCmd).Should(Exit(0))
		Expect(psCmd.OutputToString()).To(BeEmpty())

		volumesCmd := podmanTest.Podman([]string{"volume", "ls", "-q"})
		volumesCmd.WaitWithDefaultTimeout()
		Expect(volumesCmd).Should(Exit(0))
		Expect(volumesCmd.OutputToString()).To(Not(ContainSubstring(volName)))
	})
})
