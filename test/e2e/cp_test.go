package integration

import (
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

// NOTE: Only smoke tests.  The system tests (i.e., "./test/system/*") take
// care of function and regression tests.  Please consider adding system tests
// rather than e2e tests.  System tests are used in RHEL gating.

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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	// Copy a file to the container, then back to the host and make sure
	// that the contents match.
	It("podman cp file", func() {
		srcFile, err := ioutil.TempFile("", "")
		Expect(err).To(BeNil())
		defer srcFile.Close()
		defer os.Remove(srcFile.Name())

		originalContent := []byte("podman cp file test")
		err = ioutil.WriteFile(srcFile.Name(), originalContent, 0644)
		Expect(err).To(BeNil())

		// Create a container. NOTE that container mustn't be running for copying.
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		name := session.OutputToString()

		// Copy TO the container.

		// Cannot copy to a nonexistent path (note the trailing "/").
		session = podmanTest.Podman([]string{"cp", srcFile.Name(), name + ":foo/"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		// The file will now be created (and written to).
		session = podmanTest.Podman([]string{"cp", srcFile.Name(), name + ":foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Copy FROM the container.

		destFile, err := ioutil.TempFile("", "")
		Expect(err).To(BeNil())
		defer destFile.Close()
		defer os.Remove(destFile.Name())

		session = podmanTest.Podman([]string{"cp", name + ":foo", destFile.Name()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"start", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Now make sure the content matches.
		roundtripContent, err := ioutil.ReadFile(destFile.Name())
		Expect(err).To(BeNil())
		Expect(roundtripContent).To(Equal(originalContent))
	})

	// Copy a file to the container, then back to the host in --pid=host
	It("podman cp --pid=host file", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		srcFile, err := ioutil.TempFile("", "")
		Expect(err).To(BeNil())
		defer srcFile.Close()
		defer os.Remove(srcFile.Name())

		originalContent := []byte("podman cp file test")
		err = ioutil.WriteFile(srcFile.Name(), originalContent, 0644)
		Expect(err).To(BeNil())

		// Create a container. NOTE that container mustn't be running for copying.
		session := podmanTest.Podman([]string{"create", "--pid=host", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		name := session.OutputToString()

		session = podmanTest.Podman([]string{"start", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// The file will now be created (and written to).
		session = podmanTest.Podman([]string{"cp", srcFile.Name(), name + ":foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Copy FROM the container.

		destFile, err := ioutil.TempFile("", "")
		Expect(err).To(BeNil())
		defer destFile.Close()
		defer os.Remove(destFile.Name())

		session = podmanTest.Podman([]string{"cp", name + ":foo", destFile.Name()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Now make sure the content matches.
		roundtripContent, err := ioutil.ReadFile(destFile.Name())
		Expect(err).To(BeNil())
		Expect(roundtripContent).To(Equal(originalContent))
	})

	// Create a symlink in the container, use it as a copy destination and
	// make sure that the link and the resolved path are accessible and
	// give the right content.
	It("podman cp symlink", func() {
		srcFile, err := ioutil.TempFile("", "")
		Expect(err).To(BeNil())
		defer srcFile.Close()
		defer os.Remove(srcFile.Name())

		originalContent := []byte("podman cp symlink test")
		err = ioutil.WriteFile(srcFile.Name(), originalContent, 0644)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		name := session.OutputToString()

		session = podmanTest.Podman([]string{"exec", name, "ln", "-s", "/tmp", "/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"cp", "--pause=false", srcFile.Name(), name + ":/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", name, "cat", "/tmp/" + filepath.Base(srcFile.Name())})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(string(originalContent)))

		session = podmanTest.Podman([]string{"exec", name, "cat", "/test/" + filepath.Base(srcFile.Name())})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(string(originalContent)))
	})

	// Copy a file to a volume in the container.  The tricky part is that
	// containers mustn't be running for copying, so Podman has to do some
	// intense Yoga and 1) detect volume paths on the container, 2) resolve
	// the path to the volume's mount point on the host, and 3) copy the
	// data to the volume and not the container.
	It("podman cp volume", func() {
		srcFile, err := ioutil.TempFile("", "")
		Expect(err).To(BeNil())
		defer srcFile.Close()
		defer os.Remove(srcFile.Name())

		originalContent := []byte("podman cp volume")
		err = ioutil.WriteFile(srcFile.Name(), originalContent, 0644)
		Expect(err).To(BeNil())
		session := podmanTest.Podman([]string{"volume", "create", "data"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "-v", "data:/data", "--name", "container1", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"cp", srcFile.Name(), "container1" + ":/data/file.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Now get the volume's mount point, read the file and make
		// sure the contents match.
		session = podmanTest.Podman([]string{"volume", "inspect", "data", "--format", "{{.Mountpoint}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		volumeMountPoint := session.OutputToString()
		copiedContent, err := ioutil.ReadFile(filepath.Join(volumeMountPoint, "file.txt"))
		Expect(err).To(BeNil())
		Expect(copiedContent).To(Equal(originalContent))
	})

	// Create another user in the container, let them create a file, copy
	// it to the host and back to the container and make sure that we can
	// access it, and (roughly) the right users own it.
	It("podman cp from ctr chown ", func() {
		srcFile, err := ioutil.TempFile("", "")
		Expect(err).To(BeNil())
		defer srcFile.Close()
		defer os.Remove(srcFile.Name())

		setup := podmanTest.RunTopContainer("testctr")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "testctr", "adduser", "-S", "testuser"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "-u", "testuser", "testctr", "touch", "/tmp/testfile"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"cp", "--pause=false", "testctr:/tmp/testfile", srcFile.Name()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// owner of the file copied to local machine is not testuser
		u, err := user.Current()
		Expect(err).To(BeNil())
		cmd := exec.Command("ls", "-l", srcFile.Name())
		cmdRet, err := cmd.Output()
		Expect(err).To(BeNil())
		Expect(string(cmdRet)).To(ContainSubstring(u.Username))

		session = podmanTest.Podman([]string{"cp", "--pause=false", srcFile.Name(), "testctr:testfile2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// owner of the file copied to a container is the root user
		session = podmanTest.Podman([]string{"exec", "-it", "testctr", "ls", "-l", "testfile2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("root"))
	})

	// Copy the root dir "/" of a container to the host.
	It("podman cp the root directory from the ctr to an existing directory on the host ", func() {
		container := "copyroottohost"
		session := podmanTest.RunTopContainer(container)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", container, "touch", "/dummy.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		tmpDir, err := ioutil.TempDir("", "")
		Expect(err).To(BeNil())

		session = podmanTest.Podman([]string{"cp", container + ":/", tmpDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		cmd := exec.Command("ls", "-la", tmpDir)
		output, err := cmd.Output()
		lsOutput := string(output)
		Expect(err).To(BeNil())
		Expect(lsOutput).To(ContainSubstring("dummy.txt"))
		Expect(lsOutput).To(ContainSubstring("tmp"))
		Expect(lsOutput).To(ContainSubstring("etc"))
		Expect(lsOutput).To(ContainSubstring("var"))
		Expect(lsOutput).To(ContainSubstring("bin"))
		Expect(lsOutput).To(ContainSubstring("usr"))
	})
})
