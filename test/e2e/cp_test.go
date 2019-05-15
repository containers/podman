// +build !remoteclient

package integration

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman cp", func() {
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman cp file", func() {
		Skip("cp tests are broken")
		path, err := os.Getwd()
		Expect(err).To(BeNil())
		filePath := filepath.Join(path, "cp_test.txt")
		fromHostToContainer := []byte("copy from host to container")
		err = ioutil.WriteFile(filePath, fromHostToContainer, 0644)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"create", ALPINE, "cat", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		name := session.OutputToString()

		session = podmanTest.Podman([]string{"cp", filepath.Join(path, "cp_test.txt"), name + ":foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", name + ":foo", filepath.Join(path, "cp_from_container")})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		os.Remove("cp_from_container")
		os.Remove("cp_test.txt")
	})

	It("podman cp file to dir", func() {
		Skip("cp tests are broken")
		path, err := os.Getwd()
		Expect(err).To(BeNil())
		filePath := filepath.Join(path, "cp_test.txt")
		fromHostToContainer := []byte("copy from host to container directory")
		err = ioutil.WriteFile(filePath, fromHostToContainer, 0644)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"create", ALPINE, "ls", "foodir/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		name := session.OutputToString()

		session = podmanTest.Podman([]string{"cp", filepath.Join(path, "cp_test.txt"), name + ":foodir/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", name + ":foodir/cp_test.txt", path + "/receive/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		os.Remove("cp_test.txt")
		os.RemoveAll("receive")
	})

	It("podman cp dir to dir", func() {
		Skip("cp tests are broken")
		path, err := os.Getwd()
		Expect(err).To(BeNil())
		testDirPath := filepath.Join(path, "TestDir")
		err = os.Mkdir(testDirPath, 0777)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"create", ALPINE, "ls", "/foodir"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		name := session.OutputToString()

		session = podmanTest.Podman([]string{"cp", testDirPath, name + ":/foodir"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", testDirPath, name + ":/foodir"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		os.RemoveAll(testDirPath)
	})

	It("podman cp stdin/stdout", func() {
		Skip("cp tests are broken")
		path, err := os.Getwd()
		Expect(err).To(BeNil())
		testDirPath := filepath.Join(path, "TestDir")
		err = os.Mkdir(testDirPath, 0777)
		Expect(err).To(BeNil())
		cmd := exec.Command("tar", "-zcvf", "file.tar.gz", testDirPath)
		_, err = cmd.Output()
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"create", ALPINE, "ls", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		name := session.OutputToString()

		data, err := ioutil.ReadFile("foo.tar.gz")
		reader := strings.NewReader(string(data))
		cmd.Stdin = reader
		session = podmanTest.Podman([]string{"cp", "-", name + ":/foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", "file.tar.gz", name + ":/foo.tar.gz"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"cp", name + ":/foo.tar.gz", "-"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		os.Remove("file.tar.gz")
		os.RemoveAll(testDirPath)
	})
})
