//go:build !remote
// +build !remote

// build for play kube is not supported on remote yet.

package integration

import (
	"os"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman play kube with build", func() {
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

	var testYAML = `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-08-05T17:55:51Z"
  labels:
    app: foobar
  name: top_pod
spec:
  containers:
  - command:
    - top
    env:
    - name: PATH
      value: /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    - name: TERM
      value: xterm
    - name: container
      value: podman
    image: foobar
    name: foobar
    resources: {}
    securityContext:
      allowPrivilegeEscalation: true
      capabilities:
        drop:
        - CAP_MKNOD
        - CAP_NET_RAW
        - CAP_AUDIT_WRITE
      privileged: false
      readOnlyRootFilesystem: false
      seLinuxOptions: {}
    tty: true
    workingDir: /
  dnsConfig: {}
status: {}
`

	var playBuildFile = `
FROM quay.io/libpod/alpine_nginx:latest
RUN apk update && apk add strace
LABEL homer=dad
COPY copyfile /copyfile
`
	var prebuiltImage = `
FROM quay.io/libpod/alpine_nginx:latest
RUN apk update && apk add strace
LABEL marge=mom
`

	var copyFile = `just a text file
`

	It("Check that image is built using Dockerfile", func() {
		// Setup
		yamlDir := filepath.Join(tempdir, RandomString(12))
		err := os.Mkdir(yamlDir, 0755)
		Expect(err).To(BeNil(), "mkdir "+yamlDir)
		err = writeYaml(testYAML, filepath.Join(yamlDir, "top.yaml"))
		Expect(err).To(BeNil())
		app1Dir := filepath.Join(yamlDir, "foobar")
		err = os.Mkdir(app1Dir, 0755)
		Expect(err).To(BeNil())
		err = writeYaml(playBuildFile, filepath.Join(app1Dir, "Dockerfile"))
		Expect(err).To(BeNil())
		// Write a file to be copied
		err = writeYaml(copyFile, filepath.Join(app1Dir, "copyfile"))
		Expect(err).To(BeNil())
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())
		Expect(os.Chdir(yamlDir)).To(BeNil())
		defer func() { (Expect(os.Chdir(cwd)).To(BeNil())) }()

		session := podmanTest.Podman([]string{"play", "kube", "top.yaml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		exists := podmanTest.Podman([]string{"image", "exists", "foobar"})
		exists.WaitWithDefaultTimeout()
		Expect(exists).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"container", "inspect", "top_pod-foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		inspectData := inspect.InspectContainerToJSON()
		Expect(len(inspectData)).To(BeNumerically(">", 0))
		Expect(inspectData[0].Config.Labels).To(HaveKeyWithValue("homer", "dad"))
	})

	It("Check that image is built using Containerfile", func() {
		// Setup
		yamlDir := filepath.Join(tempdir, RandomString(12))
		err := os.Mkdir(yamlDir, 0755)
		Expect(err).To(BeNil(), "mkdir "+yamlDir)
		err = writeYaml(testYAML, filepath.Join(yamlDir, "top.yaml"))
		Expect(err).To(BeNil())
		app1Dir := filepath.Join(yamlDir, "foobar")
		err = os.Mkdir(app1Dir, 0755)
		Expect(err).To(BeNil())
		err = writeYaml(playBuildFile, filepath.Join(app1Dir, "Containerfile"))
		Expect(err).To(BeNil())
		// Write a file to be copied
		err = writeYaml(copyFile, filepath.Join(app1Dir, "copyfile"))
		Expect(err).To(BeNil())
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())
		Expect(os.Chdir(yamlDir)).To(BeNil())
		defer func() { (Expect(os.Chdir(cwd)).To(BeNil())) }()

		session := podmanTest.Podman([]string{"play", "kube", "top.yaml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		exists := podmanTest.Podman([]string{"image", "exists", "foobar"})
		exists.WaitWithDefaultTimeout()
		Expect(exists).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"container", "inspect", "top_pod-foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		inspectData := inspect.InspectContainerToJSON()
		Expect(len(inspectData)).To(BeNumerically(">", 0))
		Expect(inspectData[0].Config.Labels).To(HaveKeyWithValue("homer", "dad"))
	})

	It("Do not build image if already in the local store", func() {
		// Setup
		yamlDir := filepath.Join(tempdir, RandomString(12))
		err := os.Mkdir(yamlDir, 0755)
		Expect(err).To(BeNil(), "mkdir "+yamlDir)
		err = writeYaml(testYAML, filepath.Join(yamlDir, "top.yaml"))
		Expect(err).To(BeNil())

		// build an image called foobar but make sure it doesn't have
		// the same label as the yaml buildfile, so we can check that
		// the image is NOT rebuilt.
		err = writeYaml(prebuiltImage, filepath.Join(yamlDir, "Containerfile"))
		Expect(err).To(BeNil())

		app1Dir := filepath.Join(yamlDir, "foobar")
		err = os.Mkdir(app1Dir, 0755)
		Expect(err).To(BeNil())
		err = writeYaml(playBuildFile, filepath.Join(app1Dir, "Containerfile"))
		Expect(err).To(BeNil())
		// Write a file to be copied
		err = writeYaml(copyFile, filepath.Join(app1Dir, "copyfile"))
		Expect(err).To(BeNil())

		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())
		Expect(os.Chdir(yamlDir)).To(BeNil())
		defer func() { (Expect(os.Chdir(cwd)).To(BeNil())) }()

		// Build the image into the local store
		build := podmanTest.Podman([]string{"build", "-t", "foobar", "-f", "Containerfile"})
		build.WaitWithDefaultTimeout()
		Expect(build).Should(Exit(0))

		session := podmanTest.Podman([]string{"play", "kube", "top.yaml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"container", "inspect", "top_pod-foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		inspectData := inspect.InspectContainerToJSON()
		Expect(len(inspectData)).To(BeNumerically(">", 0))
		Expect(inspectData[0].Config.Labels).To(Not(HaveKey("homer")))
		Expect(inspectData[0].Config.Labels).To(HaveKeyWithValue("marge", "mom"))
	})

	It("Do not build image at all if --build=false", func() {
		// Setup
		yamlDir := filepath.Join(tempdir, RandomString(12))
		err := os.Mkdir(yamlDir, 0755)
		Expect(err).To(BeNil(), "mkdir "+yamlDir)
		err = writeYaml(testYAML, filepath.Join(yamlDir, "top.yaml"))
		Expect(err).To(BeNil())

		// build an image called foobar but make sure it doesn't have
		// the same label as the yaml buildfile, so we can check that
		// the image is NOT rebuilt.
		err = writeYaml(prebuiltImage, filepath.Join(yamlDir, "Containerfile"))
		Expect(err).To(BeNil())

		app1Dir := filepath.Join(yamlDir, "foobar")
		err = os.Mkdir(app1Dir, 0755)
		Expect(err).To(BeNil())
		err = writeYaml(playBuildFile, filepath.Join(app1Dir, "Containerfile"))
		Expect(err).To(BeNil())
		// Write a file to be copied
		err = writeYaml(copyFile, filepath.Join(app1Dir, "copyfile"))
		Expect(err).To(BeNil())

		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())
		Expect(os.Chdir(yamlDir)).To(BeNil())
		defer func() { (Expect(os.Chdir(cwd)).To(BeNil())) }()

		// Build the image into the local store
		build := podmanTest.Podman([]string{"build", "-t", "foobar", "-f", "Containerfile"})
		build.WaitWithDefaultTimeout()
		Expect(build).Should(Exit(0))

		session := podmanTest.Podman([]string{"play", "kube", "--build=false", "top.yaml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"container", "inspect", "top_pod-foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		inspectData := inspect.InspectContainerToJSON()
		Expect(len(inspectData)).To(BeNumerically(">", 0))
		Expect(inspectData[0].Config.Labels).To(Not(HaveKey("homer")))
		Expect(inspectData[0].Config.Labels).To(HaveKeyWithValue("marge", "mom"))
	})

	It("--build should override image in store", func() {
		// Setup
		yamlDir := filepath.Join(tempdir, RandomString(12))
		err := os.Mkdir(yamlDir, 0755)
		Expect(err).To(BeNil(), "os.Mkdir "+yamlDir)
		err = writeYaml(testYAML, filepath.Join(yamlDir, "top.yaml"))
		Expect(err).To(BeNil())

		// build an image called foobar but make sure it doesn't have
		// the same label as the yaml buildfile, so we can check that
		// the image is NOT rebuilt.
		err = writeYaml(prebuiltImage, filepath.Join(yamlDir, "Containerfile"))
		Expect(err).To(BeNil())

		app1Dir := filepath.Join(yamlDir, "foobar")
		err = os.Mkdir(app1Dir, 0755)
		Expect(err).To(BeNil())
		err = writeYaml(playBuildFile, filepath.Join(app1Dir, "Containerfile"))
		Expect(err).To(BeNil())
		// Write a file to be copied
		err = writeYaml(copyFile, filepath.Join(app1Dir, "copyfile"))
		Expect(err).To(BeNil())

		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())
		Expect(os.Chdir(yamlDir)).To(BeNil())
		defer func() { (Expect(os.Chdir(cwd)).To(BeNil())) }()

		// Build the image into the local store
		build := podmanTest.Podman([]string{"build", "-t", "foobar", "-f", "Containerfile"})
		build.WaitWithDefaultTimeout()
		Expect(build).Should(Exit(0))

		session := podmanTest.Podman([]string{"play", "kube", "--build", "top.yaml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"container", "inspect", "top_pod-foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		inspectData := inspect.InspectContainerToJSON()
		Expect(len(inspectData)).To(BeNumerically(">", 0))
		Expect(inspectData[0].Config.Labels).To(HaveKeyWithValue("homer", "dad"))
		Expect(inspectData[0].Config.Labels).To(Not(HaveKey("marge")))
	})

})
