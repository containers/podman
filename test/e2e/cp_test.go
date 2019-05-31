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
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman cp file", func() {
		srcPath := filepath.Join(podmanTest.RunRoot, "cp_test.txt")
		dstPath := filepath.Join(podmanTest.RunRoot, "cp_from_container")
		fromHostToContainer := []byte("copy from host to container")

		session := podmanTest.Podman([]string{"create", ALPINE, "cat", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		name := session.OutputToString()

		err := ioutil.WriteFile(srcPath, fromHostToContainer, 0644)
		Expect(err).To(BeNil())

		session = podmanTest.Podman([]string{"cp", srcPath, name + ":foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", name + ":foo", dstPath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman cp file to dir", func() {
		srcPath := filepath.Join(podmanTest.RunRoot, "cp_test.txt")
		dstDir := filepath.Join(podmanTest.RunRoot, "receive")
		fromHostToContainer := []byte("copy from host to container directory")

		session := podmanTest.Podman([]string{"create", ALPINE, "ls", "foodir/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		name := session.OutputToString()

		err := ioutil.WriteFile(srcPath, fromHostToContainer, 0644)
		Expect(err).To(BeNil())
		err = os.Mkdir(dstDir, 0755)
		Expect(err).To(BeNil())

		session = podmanTest.Podman([]string{"cp", srcPath, name + ":foodir/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", name + ":foodir/cp_test.txt", dstDir})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		os.Remove("cp_test.txt")
		os.RemoveAll("receive")
	})

	It("podman cp dir to dir", func() {
		testDirPath := filepath.Join(podmanTest.RunRoot, "TestDir")

		session := podmanTest.Podman([]string{"create", ALPINE, "ls", "/foodir"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		name := session.OutputToString()

		err := os.Mkdir(testDirPath, 0755)
		Expect(err).To(BeNil())

		session = podmanTest.Podman([]string{"cp", testDirPath, name + ":/foodir"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", testDirPath, name + ":/foodir"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman cp stdin/stdout", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		name := session.OutputToString()

		testDirPath := filepath.Join(podmanTest.RunRoot, "TestDir")
		err := os.Mkdir(testDirPath, 0755)
		Expect(err).To(BeNil())
		cmd := exec.Command("tar", "-zcvf", "file.tar.gz", testDirPath)
		_, err = cmd.Output()
		Expect(err).To(BeNil())

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
	})

	It("podman cp tar", func() {
		session := podmanTest.Podman([]string{"create", "--name", "testctr", ALPINE, "ls", "-l", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		path, err := os.Getwd()
		Expect(err).To(BeNil())
		testDirPath := filepath.Join(path, "TestDir")
		err = os.Mkdir(testDirPath, 0777)
		Expect(err).To(BeNil())
		cmd := exec.Command("tar", "-cvf", "file.tar", testDirPath)
		_, err = cmd.Output()
		Expect(err).To(BeNil())

		session = podmanTest.Podman([]string{"cp", "file.tar", "testctr:/foo/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"start", "-a", "testctr"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("file.tar"))

		os.Remove("file.tar")
		os.RemoveAll(testDirPath)
	})

	It("podman cp symlink", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		name := session.OutputToString()

		srcPath := filepath.Join(podmanTest.RunRoot, "cp_test.txt")
		fromHostToContainer := []byte("copy from host to container")
		err := ioutil.WriteFile(srcPath, fromHostToContainer, 0644)
		Expect(err).To(BeNil())

		session = podmanTest.Podman([]string{"exec", name, "ln", "-s", "/tmp", "/test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", "--pause=false", srcPath, name + ":/test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		_, err = os.Stat("/tmp/cp_test.txt")
		Expect(err).To(Not(BeNil()))
	})
})
