package integration

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman image sign", func() {
	var (
		origGNUPGHOME string
		tempdir       string
		err           error
		podmanTest    *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRemote("podman-remote image sign is not supported")
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()

		tempGNUPGHOME := filepath.Join(podmanTest.TempDir, "tmpGPG")
		err := os.Mkdir(tempGNUPGHOME, os.ModePerm)
		Expect(err).To(BeNil())

		origGNUPGHOME = os.Getenv("GNUPGHOME")
		err = os.Setenv("GNUPGHOME", tempGNUPGHOME)
		Expect(err).To(BeNil())

	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
		os.Setenv("GNUPGHOME", origGNUPGHOME)
	})

	It("podman sign image", func() {
		cmd := exec.Command("gpg", "--import", "sign/secret-key.asc")
		err := cmd.Run()
		Expect(err).To(BeNil())
		sigDir := filepath.Join(podmanTest.TempDir, "test-sign")
		err = os.MkdirAll(sigDir, os.ModePerm)
		Expect(err).To(BeNil())
		session := podmanTest.Podman([]string{"image", "sign", "--directory", sigDir, "--sign-by", "foo@bar.com", "docker://library/alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		_, err = os.Stat(filepath.Join(sigDir, "library"))
		Expect(err).To(BeNil())
	})

	It("podman sign --all multi-arch image", func() {
		cmd := exec.Command("gpg", "--import", "sign/secret-key.asc")
		err := cmd.Run()
		Expect(err).To(BeNil())
		sigDir := filepath.Join(podmanTest.TempDir, "test-sign-multi")
		err = os.MkdirAll(sigDir, os.ModePerm)
		Expect(err).To(BeNil())
		session := podmanTest.Podman([]string{"image", "sign", "--all", "--directory", sigDir, "--sign-by", "foo@bar.com", "docker://library/alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		fInfos, err := ioutil.ReadDir(filepath.Join(sigDir, "library"))
		Expect(err).To(BeNil())
		Expect(len(fInfos)).To(BeNumerically(">", 1), "len(fInfos)")
	})
})
