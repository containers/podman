// +build !remoteclient

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
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

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=/etc/ssl,tmpcopyup", ALPINE, "ls", "/etc/ssl"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("certs"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=/etc/ssl,tmpcopyup,notmpcopyup", ALPINE, "ls", "/etc/ssl"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=bind,src=/tmp,target=/tmp,tmpcopyup", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=bind,src=/tmp,target=/tmp,notmpcopyup", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=/etc/ssl,notmpcopyup", ALPINE, "ls", "/etc/ssl"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("certs")))
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
		Expect(session).To(ExitWithError())
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
		Expect(session).To(ExitWithError())
	})

	It("podman run with tmpfs named volume mounts and unmounts", func() {
		SkipIfRootless()
		volName := "testvol"
		mkVolume := podmanTest.Podman([]string{"volume", "create", "--opt", "type=tmpfs", "--opt", "device=tmpfs", "--opt", "o=nodev", "testvol"})
		mkVolume.WaitWithDefaultTimeout()
		Expect(mkVolume.ExitCode()).To(Equal(0))

		// Volume not mounted on create
		mountCmd1, err := gexec.Start(exec.Command("mount"), GinkgoWriter, GinkgoWriter)
		Expect(err).To(BeNil())
		mountCmd1.Wait(90)
		Expect(mountCmd1.ExitCode()).To(Equal(0))
		os.Stdout.Sync()
		os.Stderr.Sync()
		mountOut1 := strings.Join(strings.Fields(fmt.Sprintf("%s", mountCmd1.Out.Contents())), " ")
		fmt.Printf("Output: %s", mountOut1)
		Expect(strings.Contains(mountOut1, volName)).To(BeFalse())

		ctrName := "testctr"
		podmanSession := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, "-v", fmt.Sprintf("%s:/testvol", volName), ALPINE, "top"})
		podmanSession.WaitWithDefaultTimeout()
		Expect(podmanSession.ExitCode()).To(Equal(0))

		// Volume now mounted as container is running
		mountCmd2, err := gexec.Start(exec.Command("mount"), GinkgoWriter, GinkgoWriter)
		Expect(err).To(BeNil())
		mountCmd2.Wait(90)
		Expect(mountCmd2.ExitCode()).To(Equal(0))
		os.Stdout.Sync()
		os.Stderr.Sync()
		mountOut2 := strings.Join(strings.Fields(fmt.Sprintf("%s", mountCmd2.Out.Contents())), " ")
		fmt.Printf("Output: %s", mountOut2)
		Expect(strings.Contains(mountOut2, volName)).To(BeTrue())

		// Stop the container to unmount
		podmanStopSession := podmanTest.Podman([]string{"stop", "--timeout", "0", ctrName})
		podmanStopSession.WaitWithDefaultTimeout()
		Expect(podmanStopSession.ExitCode()).To(Equal(0))

		// We have to force cleanup so the unmount happens
		podmanCleanupSession := podmanTest.Podman([]string{"container", "cleanup", ctrName})
		podmanCleanupSession.WaitWithDefaultTimeout()
		Expect(podmanCleanupSession.ExitCode()).To(Equal(0))

		// Ensure volume is unmounted
		mountCmd3, err := gexec.Start(exec.Command("mount"), GinkgoWriter, GinkgoWriter)
		Expect(err).To(BeNil())
		mountCmd3.Wait(90)
		Expect(mountCmd3.ExitCode()).To(Equal(0))
		os.Stdout.Sync()
		os.Stderr.Sync()
		mountOut3 := strings.Join(strings.Fields(fmt.Sprintf("%s", mountCmd3.Out.Contents())), " ")
		fmt.Printf("Output: %s", mountOut3)
		Expect(strings.Contains(mountOut3, volName)).To(BeFalse())
	})

	It("podman named volume copyup", func() {
		baselineSession := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", ALPINE, "ls", "/etc/apk/"})
		baselineSession.WaitWithDefaultTimeout()
		Expect(baselineSession.ExitCode()).To(Equal(0))
		baselineOutput := baselineSession.OutputToString()

		inlineVolumeSession := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", "-v", "testvol1:/etc/apk", ALPINE, "ls", "/etc/apk/"})
		inlineVolumeSession.WaitWithDefaultTimeout()
		Expect(inlineVolumeSession.ExitCode()).To(Equal(0))
		Expect(inlineVolumeSession.OutputToString()).To(Equal(baselineOutput))

		makeVolumeSession := podmanTest.Podman([]string{"volume", "create", "testvol2"})
		makeVolumeSession.WaitWithDefaultTimeout()
		Expect(makeVolumeSession.ExitCode()).To(Equal(0))

		separateVolumeSession := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", "-v", "testvol2:/etc/apk", ALPINE, "ls", "/etc/apk/"})
		separateVolumeSession.WaitWithDefaultTimeout()
		Expect(separateVolumeSession.ExitCode()).To(Equal(0))
		Expect(separateVolumeSession.OutputToString()).To(Equal(baselineOutput))
	})

	It("podman read-only tmpfs conflict with volume", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", "--read-only", "-v", "tmp_volume:/run", ALPINE, "touch", "/run/a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"run", "--rm", "-t", "-i", "--read-only", "--tmpfs", "/run", ALPINE, "touch", "/run/a"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))
	})

	It("podman run with anonymous volume", func() {
		list1 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list1.WaitWithDefaultTimeout()
		Expect(list1.ExitCode()).To(Equal(0))
		Expect(list1.OutputToString()).To(Equal(""))

		session := podmanTest.Podman([]string{"create", "-v", "/test", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		list2 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list2.WaitWithDefaultTimeout()
		Expect(list2.ExitCode()).To(Equal(0))
		arr := list2.OutputToStringArray()
		Expect(len(arr)).To(Equal(1))
		Expect(arr[0]).To(Not(Equal("")))
	})

	It("podman rm -v removes anonymous volume", func() {
		list1 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list1.WaitWithDefaultTimeout()
		Expect(list1.ExitCode()).To(Equal(0))
		Expect(list1.OutputToString()).To(Equal(""))

		ctrName := "testctr"
		session := podmanTest.Podman([]string{"create", "--name", ctrName, "-v", "/test", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		list2 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list2.WaitWithDefaultTimeout()
		Expect(list2.ExitCode()).To(Equal(0))
		arr := list2.OutputToStringArray()
		Expect(len(arr)).To(Equal(1))
		Expect(arr[0]).To(Not(Equal("")))

		remove := podmanTest.Podman([]string{"rm", "-v", ctrName})
		remove.WaitWithDefaultTimeout()
		Expect(remove.ExitCode()).To(Equal(0))

		list3 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list3.WaitWithDefaultTimeout()
		Expect(list3.ExitCode()).To(Equal(0))
		Expect(list3.OutputToString()).To(Equal(""))
	})

	It("podman rm -v retains named volume", func() {
		list1 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list1.WaitWithDefaultTimeout()
		Expect(list1.ExitCode()).To(Equal(0))
		Expect(list1.OutputToString()).To(Equal(""))

		ctrName := "testctr"
		volName := "testvol"
		session := podmanTest.Podman([]string{"create", "--name", ctrName, "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		list2 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list2.WaitWithDefaultTimeout()
		Expect(list2.ExitCode()).To(Equal(0))
		arr := list2.OutputToStringArray()
		Expect(len(arr)).To(Equal(1))
		Expect(arr[0]).To(Equal(volName))

		remove := podmanTest.Podman([]string{"rm", "-v", ctrName})
		remove.WaitWithDefaultTimeout()
		Expect(remove.ExitCode()).To(Equal(0))

		list3 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list3.WaitWithDefaultTimeout()
		Expect(list3.ExitCode()).To(Equal(0))
		arr2 := list3.OutputToStringArray()
		Expect(len(arr2)).To(Equal(1))
		Expect(arr2[0]).To(Equal(volName))
	})

	It("podman run image volume is not noexec", func() {
		session := podmanTest.Podman([]string{"run", "--rm", redis, "grep", "/data", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("noexec")))
	})

	It("podman mount with invalid option fails", func() {
		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", "--opt", "type=tmpfs", "--opt", "device=tmpfs", "--opt", "o=invalid", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate.ExitCode()).To(Equal(0))

		volMount := podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/tmp", volName), ALPINE, "ls"})
		volMount.WaitWithDefaultTimeout()
		Expect(volMount.ExitCode()).To(Not(Equal(0)))
	})
})
