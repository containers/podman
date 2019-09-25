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

		session = podmanTest.Podman([]string{"cp", srcPath, name + ":foo/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))

		session = podmanTest.Podman([]string{"cp", srcPath, name + ":foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", name + ":foo", dstPath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"start", name})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman cp file to dir", func() {
		name := "testctr"
		setup := podmanTest.RunTopContainer(name)
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		srcPath := "/tmp/cp_test.txt"
		fromHostToContainer := []byte("copy from host to container directory")
		err := ioutil.WriteFile(srcPath, fromHostToContainer, 0644)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"exec", name, "mkdir", "foodir"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", srcPath, name + ":foodir/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"exec", name, "ls", "foodir/cp_test.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		os.Remove("/tmp/cp_test.txt")
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

		os.RemoveAll(testDirPath)
		os.Remove("file.tar.gz")
	})

	It("podman cp tar", func() {
		testctr := "testctr"
		setup := podmanTest.RunTopContainer(testctr)
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", testctr, "mkdir", "foo"})
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

		session = podmanTest.Podman([]string{"exec", testctr, "ls", "-l", "foo"})
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

		session = podmanTest.Podman([]string{"exec", name, "ln", "-s", "/tmp/nonesuch", "/test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", "--pause=false", srcPath, name + ":/test1/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))

	})
	It("podman cp volume", func() {
		session := podmanTest.Podman([]string{"volume", "create", "data"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "-v", "data:/data", "--name", "container1", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		err = ioutil.WriteFile("cp_vol", []byte("copy to the volume"), 0644)
		if err != nil {
			os.Exit(1)
		}
		session = podmanTest.Podman([]string{"cp", "cp_vol", "container1" + ":/data/cp_vol1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", "container1" + ":/data/cp_vol1", "cp_vol2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		os.Remove("cp_vol")
		os.Remove("cp_vol2")
	})

	It("podman cp from ctr chown ", func() {
		setup := podmanTest.RunTopContainer("testctr")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "testctr", "adduser", "-S", "testuser"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"exec", "-u", "testuser", "testctr", "touch", "testfile"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"cp", "--pause=false", "testctr:testfile", "testfile1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// owner of the file copied to local machine is not testuser
		cmd := exec.Command("ls", "-l", "testfile1")
		cmdRet, err := cmd.Output()
		Expect(err).To(BeNil())
		Expect(strings.Contains(string(cmdRet), "testuser")).To(BeFalse())

		session = podmanTest.Podman([]string{"cp", "--pause=false", "testfile1", "testctr:testfile2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// owner of the file copied to a container is the root user
		session = podmanTest.Podman([]string{"exec", "-it", "testctr", "ls", "-l", "testfile2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("root"))

		os.Remove("testfile1")
	})
})
