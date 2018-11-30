package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman create", func() {
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))

	})

	It("podman create container based on a local image", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		cid := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		check := podmanTest.Podman([]string{"inspect", "-l"})
		check.WaitWithDefaultTimeout()
		data := check.InspectContainerToJSON()
		Expect(data[0].ID).To(ContainSubstring(cid))
	})

	It("podman create container based on a remote image", func() {
		session := podmanTest.Podman([]string{"create", BB_GLIBC, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman create using short options", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman create adds annotation", func() {
		session := podmanTest.Podman([]string{"create", "--annotation", "HELLO=WORLD", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		check := podmanTest.Podman([]string{"inspect", "-l"})
		check.WaitWithDefaultTimeout()
		data := check.InspectContainerToJSON()
		value, ok := data[0].Config.Annotations["HELLO"]
		Expect(ok).To(BeTrue())
		Expect(value).To(Equal("WORLD"))
	})

	It("podman create --entrypoint command", func() {
		session := podmanTest.Podman([]string{"create", "--entrypoint", "/bin/foobar", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"inspect", "-l", "--format", "{{.Config.Entrypoint}}"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("/bin/foobar"))
	})

	It("podman create --entrypoint \"\"", func() {
		session := podmanTest.Podman([]string{"create", "--entrypoint", "", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"inspect", "-l", "--format", "{{.Config.Entrypoint}}"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal(""))
	})

	It("podman create --entrypoint json", func() {
		jsonString := `[ "/bin/foo", "-c"]`
		session := podmanTest.Podman([]string{"create", "--entrypoint", jsonString, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"inspect", "-l", "--format", "{{.Config.Entrypoint}}"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("/bin/foo -c"))
	})

	It("podman create --mount flag with multiple mounts", func() {
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).To(BeNil())
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")
		err = os.MkdirAll(vol2, 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"create", "--name", "test", "--mount", "type=bind,src=" + vol1 + ",target=/myvol1,z", "--mount", "type=bind,src=" + vol2 + ",target=/myvol2,z", ALPINE, "touch", "/myvol2/foo.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"logs", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).ToNot(ContainSubstring("cannot touch"))
	})

	It("podman create with --mount flag", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("skip failing test on ppc64le")
		}
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session := podmanTest.Podman([]string{"create", "--name", "test", "--rm", "--mount", fmt.Sprintf("type=bind,src=%s,target=/create/test", mountPath), ALPINE, "grep", "/create/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"logs", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/create/test rw"))

		session = podmanTest.Podman([]string{"create", "--name", "test_ro", "--rm", "--mount", fmt.Sprintf("type=bind,src=%s,target=/create/test,ro", mountPath), ALPINE, "grep", "/create/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", "test_ro"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"logs", "test_ro"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/create/test ro"))

		session = podmanTest.Podman([]string{"create", "--name", "test_shared", "--rm", "--mount", fmt.Sprintf("type=bind,src=%s,target=/create/test,shared", mountPath), ALPINE, "grep", "/create/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", "test_shared"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"logs", "test_shared"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		found, matches := session.GrepString("/create/test")
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))
		Expect(matches[0]).To(ContainSubstring("shared"))

		mountPath = filepath.Join(podmanTest.TempDir, "scratchpad")
		os.Mkdir(mountPath, 0755)
		session = podmanTest.Podman([]string{"create", "--name", "test_tmpfs", "--rm", "--mount", "type=tmpfs,target=/create/test", ALPINE, "grep", "/create/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", "test_tmpfs"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"logs", "test_tmpfs"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/create/test rw,nosuid,nodev,noexec,relatime - tmpfs"))
	})
})
