// +build !remoteclient

package integration

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

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
		path, err := os.Getwd()
		if err != nil {
			os.Exit(1)
		}
		filePath := filepath.Join(path, "cp_test.txt")
		fromHostToContainer := []byte("copy from host to container")
		err = ioutil.WriteFile(filePath, fromHostToContainer, 0644)
		if err != nil {
			os.Exit(1)
		}

		session := podmanTest.Podman([]string{"create", ALPINE, "cat", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		name := session.OutputToString()

		session = podmanTest.Podman([]string{"cp", filepath.Join(path, "cp_test.txt"), name + ":foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"start", "-a", name})
		session.WaitWithDefaultTimeout()

		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("copy from host to container"))

		session = podmanTest.Podman([]string{"cp", name + ":foo", filepath.Join(path, "cp_from_container")})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		c := exec.Command("cat", filepath.Join(path, "cp_from_container"))
		output, err := c.Output()
		if err != nil {
			os.Exit(1)
		}
		Expect(string(output)).To(Equal("copy from host to container"))
	})

	It("podman cp file to dir", func() {
		path, err := os.Getwd()
		if err != nil {
			os.Exit(1)
		}
		filePath := filepath.Join(path, "cp_test.txt")
		fromHostToContainer := []byte("copy from host to container directory")
		err = ioutil.WriteFile(filePath, fromHostToContainer, 0644)
		if err != nil {
			os.Exit(1)
		}
		session := podmanTest.Podman([]string{"create", ALPINE, "ls", "foodir/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"ps", "-a", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		name := session.OutputToString()

		session = podmanTest.Podman([]string{"cp", filepath.Join(path, "cp_test.txt"), name + ":foodir/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", "-a", name})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("cp_test.txt"))

		session = podmanTest.Podman([]string{"cp", name + ":foodir/cp_test.txt", path + "/receive/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		c := exec.Command("cat", filepath.Join(path, "receive", "cp_test.txt"))
		output, err := c.Output()
		if err != nil {
			os.Exit(1)
		}
		Expect(string(output)).To(Equal("copy from host to container directory"))
	})

	It("podman cp with .dockerignore", func() {
		path, err := os.Getwd()
		if err != nil {
			os.Exit(1)
		}

		testDirPath := filepath.Join(path, "TestDir")
		err = os.Mkdir(testDirPath, 0777)
		if err != nil {
			os.Exit(1)
		}

		testSubDirPath := filepath.Join(testDirPath, "SubTestDir")
		err = os.Mkdir(testSubDirPath, 0777)
		if err != nil {
			os.Exit(1)
		}

		filePath := filepath.Join(testSubDirPath, "subtest1.txt")
		ignoreme := []byte("test1 fail")
		err = ioutil.WriteFile(filePath, ignoreme, 0644)
		if err != nil {
			os.Exit(1)
		}

		filePath = filepath.Join(testSubDirPath, "subtest2.txt")
		notignoreme := []byte("test2 fail")
		err = ioutil.WriteFile(filePath, notignoreme, 0644)
		if err != nil {
			os.Exit(1)
		}

		filePath = filepath.Join(path, ".dockerignore")
		dockerignore := []byte("*/SubT*\n!*/*/subtest2*")
		err = ioutil.WriteFile(filePath, dockerignore, 0644)
		if err != nil {
			os.Exit(1)
		}

		session := podmanTest.Podman([]string{"create", ALPINE, "ls", "foodir/SubTestDir/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		name := session.OutputToString()

		session = podmanTest.Podman([]string{"cp", testDirPath, name + ":foodir/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", "-a", name})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("subtest2.txt"))
	})
})
