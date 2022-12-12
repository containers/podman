package integration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/buildah"
	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		Expect(session).Should(Exit(0))

		iid := session.OutputToStringArray()[len(session.OutputToStringArray())-1]

		// Verify that OS and Arch are being set
		inspect := podmanTest.Podman([]string{"inspect", iid})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectImageJSON()
		Expect(data[0]).To(HaveField("Os", runtime.GOOS))
		Expect(data[0]).To(HaveField("Architecture", runtime.GOARCH))

		session = podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman build with a secret from file", func() {
		session := podmanTest.Podman([]string{"build", "-f", "build/Containerfile.with-secret", "-t", "secret-test", "--secret", "id=mysecret,src=build/secret.txt", "build/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("somesecret"))

		session = podmanTest.Podman([]string{"rmi", "secret-test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman build with multiple secrets from files", func() {
		session := podmanTest.Podman([]string{"build", "-f", "build/Containerfile.with-multiple-secret", "-t", "multiple-secret-test", "--secret", "id=mysecret,src=build/secret.txt", "--secret", "id=mysecret2,src=build/anothersecret.txt", "build/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("somesecret"))
		Expect(session.OutputToString()).To(ContainSubstring("anothersecret"))

		session = podmanTest.Podman([]string{"rmi", "multiple-secret-test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman build with a secret from file and verify if secret file is not leaked into image", func() {
		session := podmanTest.Podman([]string{"build", "-f", "build/secret-verify-leak/Containerfile.with-secret-verify-leak", "-t", "secret-test-leak", "--secret", "id=mysecret,src=build/secret.txt", "build/secret-verify-leak"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("somesecret"))

		session = podmanTest.Podman([]string{"run", "--rm", "secret-test-leak", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("podman-build-secret")))

		session = podmanTest.Podman([]string{"rmi", "secret-test-leak"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman build with logfile", func() {
		logfile := filepath.Join(podmanTest.TempDir, "logfile")
		session := podmanTest.Podman([]string{"build", "--pull=never", "--tag", "test", "--logfile", logfile, "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Verify that OS and Arch are being set
		inspect := podmanTest.Podman([]string{"inspect", "test"})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectImageJSON()
		Expect(data[0]).To(HaveField("Os", runtime.GOOS))
		Expect(data[0]).To(HaveField("Architecture", runtime.GOARCH))

		st, err := os.Stat(logfile)
		Expect(err).ToNot(HaveOccurred())
		Expect(st.Size()).To(Not(Equal(int64(0))))

		session = podmanTest.Podman([]string{"rmi", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	// If the context directory is pointing at a file and not a directory,
	// that's a no no, fail out.
	It("podman build context directory a file", func() {
		session := podmanTest.Podman([]string{"build", "--pull=never", "build/context_dir_a_file"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	// Check that builds with different values for the squash options
	// create the appropriate number of layers, then clean up after.
	It("podman build basic alpine with squash", func() {
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/squash/Dockerfile.squash-a", "-t", "test-squash-a:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// Check for two layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(2))

		session = podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/squash/Dockerfile.squash-b", "--squash", "-t", "test-squash-b:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-b"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// Check for three layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(3))

		session = podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/squash/Dockerfile.squash-c", "--squash", "-t", "test-squash-c:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-c"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// Check for two layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(2))

		session = podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/squash/Dockerfile.squash-c", "--squash-all", "-t", "test-squash-d:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// Check for one layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(1))

		session = podmanTest.Podman([]string{"rm", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman build verify explicit cache use with squash-all and --layers", func() {
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/squash/Dockerfile.squash-c", "--squash-all", "--layers", "-t", "test-squash-d:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// Check for one layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(1))

		// Second build must use last squashed build from cache
		session = podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/squash/Dockerfile.squash-c", "--squash-all", "--layers", "-t", "test", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// Test if entire build is used from cache
		Expect(session.OutputToString()).To(ContainSubstring("Using cache"))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// Check for one layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(1))

	})

	It("podman build Containerfile locations", func() {
		// Given
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(os.TempDir())).To(Succeed())
		defer Expect(os.Chdir(cwd)).To(BeNil())

		// Write target and fake files
		targetPath, err := CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}

		fakeFile := filepath.Join(os.TempDir(), "Containerfile")
		Expect(os.WriteFile(fakeFile, []byte(fmt.Sprintf("FROM %s", ALPINE)), 0755)).To(Succeed())

		targetFile := filepath.Join(targetPath, "Containerfile")
		Expect(os.WriteFile(targetFile, []byte("FROM scratch"), 0755)).To(Succeed())

		defer func() {
			Expect(os.RemoveAll(fakeFile)).To(Succeed())
			Expect(os.RemoveAll(targetFile)).To(Succeed())
		}()

		// When
		session := podmanTest.Podman([]string{
			"build", "--pull-never", "-f", targetFile, "-t", "test-locations",
		})
		session.WaitWithDefaultTimeout()

		// Then
		Expect(session).Should(Exit(0))
		Expect(strings.Fields(session.OutputToString())).
			To(ContainElement("scratch"))
	})

	It("podman build basic alpine and print id to external file", func() {
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(os.TempDir())).To(Succeed())
		defer Expect(os.Chdir(cwd)).To(BeNil())

		targetPath, err := CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		targetFile := filepath.Join(targetPath, "idFile")

		session := podmanTest.Podman([]string{"build", "--pull-never", "build/basicalpine", "--iidfile", targetFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		id, _ := os.ReadFile(targetFile)

		// Verify that id is correct
		inspect := podmanTest.Podman([]string{"inspect", string(id)})
		inspect.WaitWithDefaultTimeout()
		data := inspect.InspectImageJSON()
		Expect("sha256:" + data[0].ID).To(Equal(string(id)))
	})

	It("podman Test PATH and reserved annotation in built image", func() {
		path := "/tmp:/bin:/usr/bin:/usr/sbin"
		session := podmanTest.Podman([]string{
			"build", "--annotation", "io.podman.annotations.seccomp=foobar", "--pull-never", "-f", "build/basicalpine/Containerfile.path", "-t", "test-path",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--name", "foobar", "test-path", "printenv", "PATH"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		stdoutLines := session.OutputToStringArray()
		Expect(stdoutLines[0]).Should(Equal(path))

		// Reserved annotation should not be applied from the image to the container.
		session = podmanTest.Podman([]string{"inspect", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).NotTo(ContainSubstring("io.podman.annotations.seccomp"))
	})

	It("podman build where workdir is a symlink and run without creating new workdir", func() {
		session := podmanTest.Podman([]string{
			"build", "-f", "build/workdir-symlink/Dockerfile", "-t", "test-symlink",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--workdir", "/tmp/link", "test-symlink"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman build http proxy test", func() {
		if env, found := os.LookupEnv("http_proxy"); found {
			defer os.Setenv("http_proxy", env)
		} else {
			defer os.Unsetenv("http_proxy")
		}
		os.Setenv("http_proxy", "1.2.3.4")
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
			// set proxy env again so it will only effect the client
			// the remote client should still use the proxy that was set for the server
			os.Setenv("http_proxy", "127.0.0.2")
		}
		podmanTest.AddImageToRWStore(ALPINE)
		dockerfile := fmt.Sprintf(`FROM %s
RUN printenv http_proxy`, ALPINE)

		dockerfilePath := filepath.Join(podmanTest.TempDir, "Dockerfile")
		err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0755)
		Expect(err).ToNot(HaveOccurred())
		// --http-proxy should be true by default so we do not set it
		session := podmanTest.Podman([]string{"build", "--pull-never", "--file", dockerfilePath, podmanTest.TempDir})
		session.Wait(120)
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("1.2.3.4"))

		// this tries to use the cache so we explicitly disable it
		session = podmanTest.Podman([]string{"build", "--no-cache", "--pull-never", "--http-proxy=false", "--file", dockerfilePath, podmanTest.TempDir})
		session.Wait(120)
		Expect(session).Should(Exit(1))
		Expect(session.ErrorToString()).To(ContainSubstring(`Error: building at STEP "RUN printenv http_proxy"`))
	})

	It("podman build relay exit code to process", func() {
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		}
		podmanTest.AddImageToRWStore(ALPINE)
		dockerfile := fmt.Sprintf(`FROM %s
RUN exit 5`, ALPINE)

		dockerfilePath := filepath.Join(podmanTest.TempDir, "Dockerfile")
		err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "-t", "error-test", "--file", dockerfilePath, podmanTest.TempDir})
		session.Wait(120)
		Expect(session).Should(Exit(5))
	})

	It("podman build and check identity", func() {
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", "build/basicalpine/Containerfile.path", "--no-cache", "-t", "test", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Verify that OS and Arch are being set
		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ index .Config.Labels }}", "test"})
		inspect.WaitWithDefaultTimeout()
		data := inspect.OutputToString()
		Expect(data).To(ContainSubstring(buildah.Version))
	})

	It("podman build and check identity with always", func() {
		// with --pull=always
		session := podmanTest.Podman([]string{"build", "--pull=always", "-f", "build/basicalpine/Containerfile.path", "--no-cache", "-t", "test1", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Verify that OS and Arch are being set
		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ index .Config.Labels }}", "test1"})
		inspect.WaitWithDefaultTimeout()
		data := inspect.OutputToString()
		Expect(data).To(ContainSubstring(buildah.Version))

		// with --pull-always
		session = podmanTest.Podman([]string{"build", "--pull-always", "-f", "build/basicalpine/Containerfile.path", "--no-cache", "-t", "test2", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Verify that OS and Arch are being set
		inspect = podmanTest.Podman([]string{"image", "inspect", "--format", "{{ index .Config.Labels }}", "test2"})
		inspect.WaitWithDefaultTimeout()
		data = inspect.OutputToString()
		Expect(data).To(ContainSubstring(buildah.Version))
	})

	It("podman-remote send correct path to copier", func() {
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		} else {
			Skip("Only valid at remote test, case works fine for regular podman and buildah")
		}
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())

		// Write target and fake files
		targetSubPath := filepath.Join(cwd, "emptydir")
		if _, err = os.Stat(targetSubPath); err != nil {
			if os.IsNotExist(err) {
				err = os.Mkdir(targetSubPath, 0755)
				Expect(err).ToNot(HaveOccurred())
			}
		}

		containerfile := fmt.Sprintf(`FROM %s
COPY /* /dir`, ALPINE)

		containerfilePath := filepath.Join(cwd, "ContainerfilePathToCopier")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "-f", "ContainerfilePathToCopier", targetSubPath})
		session.WaitWithDefaultTimeout()
		// NOTE: Docker and buildah both should error when `COPY /* /dir` is done on emptydir
		// as context. However buildkit simply ignores this so when buildah also starts ignoring
		// for such case edit this test to return 0 and check that no `/dir` should be in the result.
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("can't make relative to"))
	})

	It("podman remote test container/docker file is not inside context dir", func() {
		// Given
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())

		podmanTest.AddImageToRWStore(ALPINE)

		// Write target and fake files
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())
		targetSubPath := filepath.Join(targetPath, "subdir")
		err = os.Mkdir(targetSubPath, 0755)
		Expect(err).ToNot(HaveOccurred())
		dummyFile := filepath.Join(targetSubPath, "dummy")
		err = os.WriteFile(dummyFile, []byte("dummy"), 0644)
		Expect(err).ToNot(HaveOccurred())

		containerfile := fmt.Sprintf(`FROM %s
ADD . /test
RUN find /test`, ALPINE)

		containerfilePath := filepath.Join(targetPath, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).ToNot(HaveOccurred())

		defer func() {
			Expect(os.Chdir(cwd)).To(Succeed())
			Expect(os.RemoveAll(targetPath)).To(Succeed())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(Succeed())

		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "-f", "Containerfile", targetSubPath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("/test/dummy"))
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
		Expect(err).ToNot(HaveOccurred())

		podmanTest.AddImageToRWStore(ALPINE)

		// Write target and fake files
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())
		targetSubPath := filepath.Join(targetPath, "subdir")
		err = os.Mkdir(targetSubPath, 0755)
		Expect(err).ToNot(HaveOccurred())

		containerfile := fmt.Sprintf("FROM %s", ALPINE)

		containerfilePath := filepath.Join(targetSubPath, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).ToNot(HaveOccurred())

		defer func() {
			Expect(os.Chdir(cwd)).To(Succeed())
			Expect(os.RemoveAll(targetPath)).To(Succeed())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(Succeed())

		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "-f", "subdir/Containerfile", "."})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
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
		Expect(err).ToNot(HaveOccurred())

		podmanTest.AddImageToRWStore(ALPINE)

		// Write target and fake files
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())

		containerfile := fmt.Sprintf(`FROM %s
ADD . /testfilter/
RUN find /testfilter/`, ALPINE)

		containerfilePath := filepath.Join(targetPath, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).ToNot(HaveOccurred())

		targetSubPath := filepath.Join(targetPath, "subdir")
		err = os.Mkdir(targetSubPath, 0755)
		Expect(err).ToNot(HaveOccurred())

		dummyFile1 := filepath.Join(targetPath, "dummy1")
		err = os.WriteFile(dummyFile1, []byte("dummy1"), 0644)
		Expect(err).ToNot(HaveOccurred())

		dummyFile2 := filepath.Join(targetPath, "dummy2")
		err = os.WriteFile(dummyFile2, []byte("dummy2"), 0644)
		Expect(err).ToNot(HaveOccurred())

		dummyFile3 := filepath.Join(targetSubPath, "dummy3")
		err = os.WriteFile(dummyFile3, []byte("dummy3"), 0644)
		Expect(err).ToNot(HaveOccurred())

		defer func() {
			Expect(os.Chdir(cwd)).To(Succeed())
			Expect(os.RemoveAll(targetPath)).To(Succeed())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(Succeed())

		dockerignoreContent := `dummy1
subdir**`
		dockerignoreFile := filepath.Join(targetPath, ".dockerignore")

		// test .dockerignore
		By("Test .dockererignore")
		err = os.WriteFile(dockerignoreFile, []byte(dockerignoreContent), 0644)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"build", "-t", "test", "."})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output := session.OutputToString()
		Expect(output).To(ContainSubstring("/testfilter/dummy2"))
		Expect(output).NotTo(ContainSubstring("/testfilter/dummy1"))
		Expect(output).NotTo(ContainSubstring("/testfilter/subdir"))
	})

	// See https://github.com/containers/podman/issues/13535
	It("Remote build .containerignore filtering embedded directory (#13535)", func() {
		SkipIfNotRemote("Testing remote .containerignore file filtering")
		Skip("FIXME: #15014: test times out in 'dd' on f36.")

		podmanTest.RestartRemoteService()

		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())

		podmanTest.AddImageToRWStore(ALPINE)

		contents := bytes.Buffer{}
		contents.WriteString("FROM " + ALPINE + "\n")
		contents.WriteString("ADD . /testfilter/\n")
		contents.WriteString("RUN find /testfilter/ -print\n")

		containerfile := filepath.Join(tempdir, "Containerfile")
		Expect(os.WriteFile(containerfile, contents.Bytes(), 0644)).ToNot(HaveOccurred())

		contextDir, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(contextDir)

		Expect(os.WriteFile(filepath.Join(contextDir, "expected"), contents.Bytes(), 0644)).
			ToNot(HaveOccurred())

		subdirPath := filepath.Join(contextDir, "subdir")
		Expect(os.MkdirAll(subdirPath, 0755)).ToNot(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(subdirPath, "extra"), contents.Bytes(), 0644)).
			ToNot(HaveOccurred())
		randomFile := filepath.Join(subdirPath, "randomFile")
		dd := exec.Command("dd", "if=/dev/urandom", "of="+randomFile, "bs=1G", "count=1")
		ddSession, err := Start(dd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(ddSession, "10s", "1s").Should(Exit(0))

		// make cwd as context root path
		Expect(os.Chdir(contextDir)).ToNot(HaveOccurred())
		defer func() {
			err := os.Chdir(cwd)
			Expect(err).ToNot(HaveOccurred())
		}()

		By("Test .containerignore filtering subdirectory")
		err = os.WriteFile(filepath.Join(contextDir, ".containerignore"), []byte(`subdir/`), 0644)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"build", "-f", containerfile, contextDir})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Exit(0))

		output := session.OutputToString()
		Expect(output).To(ContainSubstring("Containerfile"))
		Expect(output).To(ContainSubstring("/testfilter/expected"))
		Expect(output).NotTo(ContainSubstring("subdir"))
	})

	It("podman remote test context dir contains empty dirs and symlinks", func() {
		SkipIfNotRemote("Testing remote contextDir empty")
		podmanTest.RestartRemoteService()

		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())

		podmanTest.AddImageToRWStore(ALPINE)

		// Write target and fake files
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())
		targetSubPath := filepath.Join(targetPath, "subdir")
		err = os.Mkdir(targetSubPath, 0755)
		Expect(err).ToNot(HaveOccurred())
		dummyFile := filepath.Join(targetSubPath, "dummy")
		err = os.WriteFile(dummyFile, []byte("dummy"), 0644)
		Expect(err).ToNot(HaveOccurred())

		emptyDir := filepath.Join(targetSubPath, "emptyDir")
		err = os.Mkdir(emptyDir, 0755)
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(targetSubPath)).To(Succeed())
		Expect(os.Symlink("dummy", "dummy-symlink")).To(Succeed())

		containerfile := fmt.Sprintf(`FROM %s
ADD . /test
RUN find /test
RUN [[ -L /test/dummy-symlink ]] && echo SYMLNKOK || echo SYMLNKERR`, ALPINE)

		containerfilePath := filepath.Join(targetSubPath, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).ToNot(HaveOccurred())

		defer func() {
			Expect(os.Chdir(cwd)).To(Succeed())
			Expect(os.RemoveAll(targetPath)).To(Succeed())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(Succeed())

		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", targetSubPath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("/test/dummy"))
		Expect(session.OutputToString()).To(ContainSubstring("/test/emptyDir"))
		Expect(session.OutputToString()).To(ContainSubstring("/test/dummy-symlink"))
		Expect(session.OutputToString()).To(ContainSubstring("SYMLNKOK"))
	})

	It("podman build --from, --add-host, --cap-drop, --cap-add", func() {
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())

		containerFile := filepath.Join(targetPath, "Containerfile")
		content := `FROM scratch
RUN cat /etc/hosts
RUN grep CapEff /proc/self/status`

		Expect(os.WriteFile(containerFile, []byte(content), 0755)).To(Succeed())

		defer func() {
			Expect(os.RemoveAll(containerFile)).To(Succeed())
		}()

		// When
		session := podmanTest.Podman([]string{
			"build", "--pull-never", "--cap-drop=all", "--cap-add=net_bind_service", "--add-host", "testhost:1.2.3.4", "--from", ALPINE, targetPath,
		})
		session.WaitWithDefaultTimeout()

		// Then
		Expect(session).Should(Exit(0))
		Expect(strings.Fields(session.OutputToString())).
			To(ContainElement(ALPINE))
		Expect(strings.Fields(session.OutputToString())).
			To(ContainElement("testhost"))
		Expect(strings.Fields(session.OutputToString())).
			To(ContainElement("0000000000000400"))
	})

	It("podman build --isolation && --arch", func() {
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())

		containerFile := filepath.Join(targetPath, "Containerfile")
		Expect(os.WriteFile(containerFile, []byte(fmt.Sprintf("FROM %s", ALPINE)), 0755)).To(Succeed())

		defer func() {
			Expect(os.RemoveAll(containerFile)).To(Succeed())
		}()

		// When
		session := podmanTest.Podman([]string{
			"build", "--isolation", "oci", "--arch", "arm64", targetPath,
		})
		session.WaitWithDefaultTimeout()
		// Then
		Expect(session).Should(Exit(0))

		// When
		session = podmanTest.Podman([]string{
			"build", "--isolation", "chroot", "--arch", "arm64", targetPath,
		})
		session.WaitWithDefaultTimeout()
		// Then
		Expect(session).Should(Exit(0))

		// When
		session = podmanTest.Podman([]string{
			"build", "--pull-never", "--isolation", "rootless", "--arch", "arm64", targetPath,
		})
		session.WaitWithDefaultTimeout()
		// Then
		Expect(session).Should(Exit(0))

		// When
		session = podmanTest.Podman([]string{
			"build", "--pull-never", "--isolation", "bogus", "--arch", "arm64", targetPath,
		})
		session.WaitWithDefaultTimeout()
		// Then
		Expect(session).Should(Exit(125))
	})

	It("podman build --timestamp flag", func() {
		containerfile := fmt.Sprintf(`FROM %s
RUN echo hello`, ALPINE)

		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := os.WriteFile(containerfilePath, []byte(containerfile), 0755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--timestamp", "0", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Created }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("1970-01-01 00:00:00 +0000 UTC"))
	})

	It("podman build --log-rusage", func() {
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())

		containerFile := filepath.Join(targetPath, "Containerfile")
		content := `FROM scratch`

		Expect(os.WriteFile(containerFile, []byte(content), 0755)).To(Succeed())

		session := podmanTest.Podman([]string{"build", "--log-rusage", "--pull-never", targetPath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("(system)"))
		Expect(session.OutputToString()).To(ContainSubstring("(user)"))
		Expect(session.OutputToString()).To(ContainSubstring("(elapsed)"))
	})

	It("podman build --arch --os flag", func() {
		containerfile := `FROM scratch`
		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := os.WriteFile(containerfilePath, []byte(containerfile), 0755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--arch", "foo", "--os", "bar", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

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
		err := os.WriteFile(containerfilePath, []byte(containerfile), 0755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--os", "windows", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Architecture }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal(runtime.GOARCH))

		inspect = podmanTest.Podman([]string{"image", "inspect", "--format", "{{ .Os }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("windows"))

	})

	It("podman build device test", func() {
		if _, err := os.Lstat("/dev/fuse"); err != nil {
			Skip(fmt.Sprintf("test requires stat /dev/fuse to work: %v", err))
		}
		containerfile := fmt.Sprintf(`FROM %s
RUN ls /dev/fuse`, ALPINE)
		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := os.WriteFile(containerfilePath, []byte(containerfile), 0755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))

		session = podmanTest.Podman([]string{"build", "--pull-never", "--device", "/dev/fuse", "-t", "test", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman build device rename test", func() {
		SkipIfRootless("rootless builds do not currently support renaming devices")
		containerfile := fmt.Sprintf(`FROM %s
RUN ls /dev/test1`, ALPINE)
		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := os.WriteFile(containerfilePath, []byte(containerfile), 0755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "--pull-never", "-t", "test", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))

		session = podmanTest.Podman([]string{"build", "--pull-never", "--device", "/dev/zero:/dev/test1", "-t", "test", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman build use absolute path even if given relative", func() {
		containerFile := fmt.Sprintf(`FROM %s`, ALPINE)
		relativeDir := filepath.Join(podmanTest.TempDir, "relativeDir")
		containerFilePath := filepath.Join(relativeDir, "Containerfile")
		buildRoot := filepath.Join(relativeDir, "build-root")

		err = os.Mkdir(relativeDir, 0755)
		Expect(err).ToNot(HaveOccurred())
		err = os.Mkdir(buildRoot, 0755)
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(containerFilePath, []byte(containerFile), 0755)
		Expect(err).ToNot(HaveOccurred())
		build := podmanTest.Podman([]string{"build", "-f", containerFilePath, buildRoot})
		build.WaitWithDefaultTimeout()
		Expect(build).To(Exit(0))
	})
})
