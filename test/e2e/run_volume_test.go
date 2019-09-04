// +build !remoteclient

package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run with volumes", func() {
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

	It("podman run with volume flag", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session := podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/run/test", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		found, matches := session.GrepString("/run/test")
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))

		mountPath = filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/run/test:ro", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		found, matches = session.GrepString("/run/test")
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("ro"))

		mountPath = filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/run/test:shared", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		found, matches = session.GrepString("/run/test")
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))
		Expect(matches[0]).To(ContainSubstring("shared"))

		// Cached is ignored
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/run/test:cached", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		found, matches = session.GrepString("/run/test")
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))
		Expect(matches[0]).To(Not(ContainSubstring("cached")))

		// Delegated is ignored
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/run/test:delegated", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		found, matches = session.GrepString("/run/test")
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))
		Expect(matches[0]).To(Not(ContainSubstring("delegated")))
	})

	It("podman run with --mount flag", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("skip failing test on ppc64le")
		}
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session := podmanTest.Podman([]string{"run", "--rm", "--mount", fmt.Sprintf("type=bind,src=%s,target=/run/test", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/run/test rw"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", fmt.Sprintf("type=bind,src=%s,target=/run/test,ro", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/run/test ro"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", fmt.Sprintf("type=bind,src=%s,target=/run/test,shared", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		found, matches := session.GrepString("/run/test")
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(ContainSubstring("rw"))
		Expect(matches[0]).To(ContainSubstring("shared"))

		mountPath = filepath.Join(podmanTest.TempDir, "scratchpad")
		os.Mkdir(mountPath, 0755)
		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=/run/test", ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/run/test rw,nosuid,nodev,noexec,relatime - tmpfs"))
	})

	It("podman run with conflicting volumes errors", func() {
		mountPath := filepath.Join(podmanTest.TmpDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/run/test", mountPath), "-v", "/tmp:/run/test", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman run with conflict between image volume and user mount succeeds", func() {
		podmanTest.RestoreArtifact(redis)
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err := os.Mkdir(mountPath, 0755)
		Expect(err).To(BeNil())
		testFile := filepath.Join(mountPath, "test1")
		f, err := os.Create(testFile)
		f.Close()
		Expect(err).To(BeNil())
		session := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/data", mountPath), redis, "ls", "/data/test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run with mount flag and boolean options", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session := podmanTest.Podman([]string{"run", "--rm", "--mount", fmt.Sprintf("type=bind,src=%s,target=/run/test,ro=false", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/run/test rw"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", fmt.Sprintf("type=bind,src=%s,target=/run/test,ro=true", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/run/test ro"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", fmt.Sprintf("type=bind,src=%s,target=/run/test,ro=true,rw=false", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman run with volume flag and multiple named volumes", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "-v", "testvol1:/testvol1", "-v", "testvol2:/testvol2", ALPINE, "grep", "/testvol", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("/testvol1"))
		Expect(session.OutputToString()).To(ContainSubstring("/testvol2"))
	})

	It("podman run with volumes and suid/dev/exec options", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		os.Mkdir(mountPath, 0755)
		session := podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/run/test:suid,dev,exec", mountPath), ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		found, matches := session.GrepString("/run/test")
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(Not(ContainSubstring("noexec")))
		Expect(matches[0]).To(Not(ContainSubstring("nodev")))
		Expect(matches[0]).To(Not(ContainSubstring("nosuid")))

		session = podmanTest.Podman([]string{"run", "--rm", "--tmpfs", "/run/test:suid,dev,exec", ALPINE, "grep", "/run/test", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		found, matches = session.GrepString("/run/test")
		Expect(found).Should(BeTrue())
		Expect(matches[0]).To(Not(ContainSubstring("noexec")))
		Expect(matches[0]).To(Not(ContainSubstring("nodev")))
		Expect(matches[0]).To(Not(ContainSubstring("nosuid")))
	})

	It("podman run with noexec can't exec", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "-v", "/bin:/hostbin:noexec", ALPINE, "/hostbin/ls", "/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})
})
