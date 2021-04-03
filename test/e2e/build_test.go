package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/buildah"
	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman build", func() {
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

	// Let's first do the most simple build possible to make sure stuff is
	// happy and then clean up after ourselves to make sure that works too.
	It("podman build and remove basic alpine", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"build", "--pull-never", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		iid := session.OutputToStringArray()[len(session.OutputToStringArray())-1]

		// Verify that OS and Arch are being set
		inspect := podmanTest.Podman([]string{"inspect", iid})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectImageJSON()
		Expect(data[0].Os).To(Equal(runtime.GOOS))
		Expect(data[0].Architecture).To(Equal(runtime.GOARCH))

		session = podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman build with logfile", func() {
		logfile := filepath.Join(podmanTest.TempDir, "logfile")
		session := podmanTest.Podman([]string{"build", "--pull-never", "--tag", "test", "--logfile", logfile, "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Verify that OS and Arch are being set
		inspect := podmanTest.Podman([]string{"inspect", "test"})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectImageJSON()
		Expect(data[0].Os).To(Equal(runtime.GOOS))
		Expect(data[0].Architecture).To(Equal(runtime.GOARCH))

		st, err := os.Stat(logfile)
		Expect(err).To(BeNil())
		Expect(st.Size()).To(Not(Equal(int64(0))))

		session = podmanTest.Podman([]string{"rmi", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	// If the context directory is pointing at a file and not a directory,
	// that's a no no, fail out.
	It("podman build context directory a file", func() {
		session := podmanTest.Podman([]string{"build", "--pull-never", "build/context_dir_a_file"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	// Check that builds with different values for the squash options
	// create the appropriate number of layers, then clean up after.
	It("podman build basic alpine with squash", func() {
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/squash/Dockerfile.squash-a", "-t", "test-squash-a:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// Check for two layers
		Expect(len(strings.Fields(session.OutputToString()))).To(Equal(2))

		session = podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/squash/Dockerfile.squash-b", "--squash", "-t", "test-squash-b:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-b"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// Check for three layers
		Expect(len(strings.Fields(session.OutputToString()))).To(Equal(3))

		session = podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/squash/Dockerfile.squash-c", "--squash", "-t", "test-squash-c:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-c"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// Check for two layers
		Expect(len(strings.Fields(session.OutputToString()))).To(Equal(2))

		session = podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/squash/Dockerfile.squash-c", "--squash-all", "-t", "test-squash-d:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-d"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// Check for one layers
		Expect(len(strings.Fields(session.OutputToString()))).To(Equal(1))

		session = podmanTest.Podman([]string{"rm", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman build Containerfile locations", func() {
		// Given
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())
		Expect(os.Chdir(os.TempDir())).To(BeNil())
		defer Expect(os.Chdir(cwd)).To(BeNil())

		// Write target and fake files
		targetPath, err := CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}

		fakeFile := filepath.Join(os.TempDir(), "Containerfile")
		Expect(ioutil.WriteFile(fakeFile, []byte(fmt.Sprintf("FROM %s", ALPINE)), 0755)).To(BeNil())

		targetFile := filepath.Join(targetPath, "Containerfile")
		Expect(ioutil.WriteFile(targetFile, []byte("FROM scratch"), 0755)).To(BeNil())

		defer func() {
			Expect(os.RemoveAll(fakeFile)).To(BeNil())
			Expect(os.RemoveAll(targetFile)).To(BeNil())
		}()

		// When
		session := podmanTest.Podman([]string{
			"build", "--pull-never", "-f", targetFile, "-t", "test-locations",
		})
		session.WaitWithDefaultTimeout()

		// Then
		Expect(session.ExitCode()).To(Equal(0))
		Expect(strings.Fields(session.OutputToString())).
			To(ContainElement("scratch"))
	})

	It("podman build basic alpine and print id to external file", func() {
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())
		Expect(os.Chdir(os.TempDir())).To(BeNil())
		defer Expect(os.Chdir(cwd)).To(BeNil())

		targetPath, err := CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		targetFile := filepath.Join(targetPath, "idFile")

		session := podmanTest.Podman([]string{"build", "--pull-never", "build/basicalpine", "--iidfile", targetFile})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		id, _ := ioutil.ReadFile(targetFile)

		// Verify that id is correct
		inspect := podmanTest.Podman([]string{"inspect", string(id)})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectImageJSON()
		Expect("sha256:" + data[0].ID).To(Equal(string(id)))
	})

	It("podman Test PATH in built image", func() {
		path := "/tmp:/bin:/usr/bin:/usr/sbin"
		session := podmanTest.Podman([]string{
			"build", "--pull-never", "-f", "build/basicalpine/Containerfile.path", "-t", "test-path",
		})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "test-path", "printenv", "PATH"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		stdoutLines := session.OutputToStringArray()
		Expect(stdoutLines[0]).Should(Equal(path))
	})

	It("podman build --http_proxy flag", func() {
		os.Setenv("http_proxy", "1.2.3.4")
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		}
		podmanTest.AddImageToRWStore(ALPINE)
		dockerfile := fmt.Sprintf(`FROM %s
RUN printenv http_proxy`, ALPINE)

		dockerfilePath := filepath.Join(podmanTest.TempDir, "Dockerfile")
		err := ioutil.WriteFile(dockerfilePath, []byte(dockerfile), 0755)
		Expect(err).To(BeNil())
		session := podmanTest.Podman([]string{"build", "--pull-never", "--http-proxy", "--file", dockerfilePath, podmanTest.TempDir})
		session.Wait(120)
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("1.2.3.4")
		Expect(ok).To(BeTrue())
		os.Unsetenv("http_proxy")
	})

	It("podman build and check identity", func() {
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/basicalpine/Containerfile.path", "--no-cache", "-t", "test", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Verify that OS and Arch are being set
		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ index .Config.Labels }}", "test"})
		inspect.WaitWithDefaultTimeout()
		data := inspect.OutputToString()
		Expect(data).To(ContainSubstring(buildah.Version))
	})

	It("podman remote test container/docker file is not inside context dir", func() {
		// Given
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())

		podmanTest.AddImageToRWStore(ALPINE)

		// Write target and fake files
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())
		targetSubPath := filepath.Join(targetPath, "subdir")
		err = os.Mkdir(targetSubPath, 0755)
		Expect(err).To(BeNil())
		dummyFile := filepath.Join(targetSubPath, "dummy")
		err = ioutil.WriteFile(dummyFile, []byte("dummy"), 0644)
		Expect(err).To(BeNil())

		containerfile := fmt.Sprintf(`FROM %s
ADD . /test
RUN find /test`, ALPINE)

		containerfilePath := filepath.Join(targetPath, "Containerfile")
		err = ioutil.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).To(BeNil())

		defer func() {
			Expect(os.Chdir(cwd)).To(BeNil())
			Expect(os.RemoveAll(targetPath)).To(BeNil())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(BeNil())

		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "-f", "Containerfile", targetSubPath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("/test/dummy")
		Expect(ok).To(BeTrue())
	})

	It("podman remote test container/docker file is not at root of context dir", func() {
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		} else {
			Skip("Only valid at remote test")
		}
		// Given
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())

		podmanTest.AddImageToRWStore(ALPINE)

		// Write target and fake files
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())
		targetSubPath := filepath.Join(targetPath, "subdir")
		err = os.Mkdir(targetSubPath, 0755)
		Expect(err).To(BeNil())

		containerfile := fmt.Sprintf("FROM %s", ALPINE)

		containerfilePath := filepath.Join(targetSubPath, "Containerfile")
		err = ioutil.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).To(BeNil())

		defer func() {
			Expect(os.Chdir(cwd)).To(BeNil())
			Expect(os.RemoveAll(targetPath)).To(BeNil())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(BeNil())

		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "-f", "subdir/Containerfile", "."})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman remote test .dockerignore", func() {
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		} else {
			Skip("Only valid at remote test")
		}
		// Given
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())

		podmanTest.AddImageToRWStore(ALPINE)

		// Write target and fake files
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())

		containerfile := fmt.Sprintf(`FROM %s
ADD . /testfilter/
RUN find /testfilter/`, ALPINE)

		containerfilePath := filepath.Join(targetPath, "Containerfile")
		err = ioutil.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).To(BeNil())

		targetSubPath := filepath.Join(targetPath, "subdir")
		err = os.Mkdir(targetSubPath, 0755)
		Expect(err).To(BeNil())

		dummyFile1 := filepath.Join(targetPath, "dummy1")
		err = ioutil.WriteFile(dummyFile1, []byte("dummy1"), 0644)
		Expect(err).To(BeNil())

		dummyFile2 := filepath.Join(targetPath, "dummy2")
		err = ioutil.WriteFile(dummyFile2, []byte("dummy2"), 0644)
		Expect(err).To(BeNil())

		dummyFile3 := filepath.Join(targetSubPath, "dummy3")
		err = ioutil.WriteFile(dummyFile3, []byte("dummy3"), 0644)
		Expect(err).To(BeNil())

		defer func() {
			Expect(os.Chdir(cwd)).To(BeNil())
			Expect(os.RemoveAll(targetPath)).To(BeNil())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(BeNil())

		dockerignoreContent := `dummy1
subdir**`
		dockerignoreFile := filepath.Join(targetPath, ".dockerignore")

		// test .dockerignore
		By("Test .dockererignore")
		err = ioutil.WriteFile(dockerignoreFile, []byte(dockerignoreContent), 0644)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"build", "-t", "test", "."})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("/testfilter/dummy1")
		Expect(ok).NotTo(BeTrue())
		ok, _ = session.GrepString("/testfilter/dummy2")
		Expect(ok).To(BeTrue())
		ok, _ = session.GrepString("/testfilter/subdir")
		Expect(ok).NotTo(BeTrue()) //.dockerignore filters both subdir and inside subdir
		ok, _ = session.GrepString("/testfilter/subdir/dummy3")
		Expect(ok).NotTo(BeTrue())
	})

	It("podman remote test context dir contains empty dirs and symlinks", func() {
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		} else {
			Skip("Only valid at remote test")
		}
		// Given
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())

		podmanTest.AddImageToRWStore(ALPINE)

		// Write target and fake files
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())
		targetSubPath := filepath.Join(targetPath, "subdir")
		err = os.Mkdir(targetSubPath, 0755)
		Expect(err).To(BeNil())
		dummyFile := filepath.Join(targetSubPath, "dummy")
		err = ioutil.WriteFile(dummyFile, []byte("dummy"), 0644)
		Expect(err).To(BeNil())

		emptyDir := filepath.Join(targetSubPath, "emptyDir")
		err = os.Mkdir(emptyDir, 0755)
		Expect(err).To(BeNil())
		Expect(os.Chdir(targetSubPath)).To(BeNil())
		Expect(os.Symlink("dummy", "dummy-symlink")).To(BeNil())

		containerfile := fmt.Sprintf(`FROM %s
ADD . /test
RUN find /test
RUN [[ -L /test/dummy-symlink ]] && echo SYMLNKOK || echo SYMLNKERR`, ALPINE)

		containerfilePath := filepath.Join(targetSubPath, "Containerfile")
		err = ioutil.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).To(BeNil())

		defer func() {
			Expect(os.Chdir(cwd)).To(BeNil())
			Expect(os.RemoveAll(targetPath)).To(BeNil())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(BeNil())

		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", targetSubPath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("/test/dummy")
		Expect(ok).To(BeTrue())
		ok, _ = session.GrepString("/test/emptyDir")
		Expect(ok).To(BeTrue())
		ok, _ = session.GrepString("/test/dummy-symlink")
		Expect(ok).To(BeTrue())
		ok, _ = session.GrepString("SYMLNKOK")
		Expect(ok).To(BeTrue())
	})

	It("podman build --from, --add-host, --cap-drop, --cap-add", func() {
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())

		containerFile := filepath.Join(targetPath, "Containerfile")
		content := `FROM scratch
RUN cat /etc/hosts
RUN grep CapEff /proc/self/status`

		Expect(ioutil.WriteFile(containerFile, []byte(content), 0755)).To(BeNil())

		defer func() {
			Expect(os.RemoveAll(containerFile)).To(BeNil())
		}()

		// When
		session := podmanTest.Podman([]string{
			"build", "--pull-never", "--cap-drop=all", "--cap-add=net_bind_service", "--add-host", "testhost:1.2.3.4", "--from", ALPINE, targetPath,
		})
		session.WaitWithDefaultTimeout()

		// Then
		Expect(session.ExitCode()).To(Equal(0))
		Expect(strings.Fields(session.OutputToString())).
			To(ContainElement(ALPINE))
		Expect(strings.Fields(session.OutputToString())).
			To(ContainElement("testhost"))
		Expect(strings.Fields(session.OutputToString())).
			To(ContainElement("0000000000000400"))
	})

	It("podman build --isolation && --arch", func() {
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())

		containerFile := filepath.Join(targetPath, "Containerfile")
		Expect(ioutil.WriteFile(containerFile, []byte(fmt.Sprintf("FROM %s", ALPINE)), 0755)).To(BeNil())

		defer func() {
			Expect(os.RemoveAll(containerFile)).To(BeNil())
		}()

		// When
		session := podmanTest.Podman([]string{
			"build", "--isolation", "oci", "--arch", "arm64", targetPath,
		})
		session.WaitWithDefaultTimeout()
		// Then
		Expect(session.ExitCode()).To(Equal(0))

		// When
		session = podmanTest.Podman([]string{
			"build", "--isolation", "chroot", "--arch", "arm64", targetPath,
		})
		session.WaitWithDefaultTimeout()
		// Then
		Expect(session.ExitCode()).To(Equal(0))

		// When
		session = podmanTest.Podman([]string{
			"build", "--pull-never", "--isolation", "rootless", "--arch", "arm64", targetPath,
		})
		session.WaitWithDefaultTimeout()
		// Then
		Expect(session.ExitCode()).To(Equal(0))

		// When
		session = podmanTest.Podman([]string{
			"build", "--pull-never", "--isolation", "bogus", "--arch", "arm64", targetPath,
		})
		session.WaitWithDefaultTimeout()
		// Then
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman build --timestamp flag", func() {
		containerfile := fmt.Sprintf(`FROM %s
RUN echo hello`, ALPINE)

		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := ioutil.WriteFile(containerfilePath, []byte(containerfile), 0755)
		Expect(err).To(BeNil())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--timestamp", "0", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Created }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("1970-01-01 00:00:00 +0000 UTC"))
	})

	It("podman build --log-rusage", func() {
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())

		containerFile := filepath.Join(targetPath, "Containerfile")
		content := `FROM scratch`

		Expect(ioutil.WriteFile(containerFile, []byte(content), 0755)).To(BeNil())

		session := podmanTest.Podman([]string{"build", "--log-rusage", "--pull-never", targetPath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("(system)"))
		Expect(session.OutputToString()).To(ContainSubstring("(user)"))
		Expect(session.OutputToString()).To(ContainSubstring("(elapsed)"))
	})

	It("podman build --arch --os flag", func() {
		containerfile := `FROM scratch`
		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := ioutil.WriteFile(containerfilePath, []byte(containerfile), 0755)
		Expect(err).To(BeNil())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--arch", "foo", "--os", "bar", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Architecture }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("foo"))

		inspect = podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Os }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("bar"))

	})

	It("podman build --os windows flag", func() {
		containerfile := `FROM scratch`
		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := ioutil.WriteFile(containerfilePath, []byte(containerfile), 0755)
		Expect(err).To(BeNil())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--os", "windows", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Architecture }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal(runtime.GOARCH))

		inspect = podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Os }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("windows"))

	})
})
