package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
)

var _ = Describe("Podman privileged container tests", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()

	})

	It("podman privileged make sure sys is mounted rw", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "busybox", "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, lines := session.GrepString("sysfs")
		Expect(ok).To(BeTrue())
		Expect(lines[0]).To(ContainSubstring("sysfs (rw,"))
	})

	It("podman privileged CapEff", func() {
		cap := podmanTest.SystemExec("grep", []string{"CapEff", "/proc/self/status"})
		cap.WaitWithDefaultTimeout()
		Expect(cap.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "--privileged", "busybox", "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal(cap.OutputToString()))
	})

	It("podman cap-add CapEff", func() {
		cap := podmanTest.SystemExec("grep", []string{"CapEff", "/proc/self/status"})
		cap.WaitWithDefaultTimeout()
		Expect(cap.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "--cap-add", "all", "busybox", "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal(cap.OutputToString()))
	})

	It("podman cap-drop CapEff", func() {
		session := podmanTest.Podman([]string{"run", "--cap-drop", "all", "busybox", "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		capEff := strings.Split(session.OutputToString(), " ")
		Expect("0000000000000000").To(Equal(capEff[1]))
	})

	It("podman non-privileged should have very few devices", func() {
		session := podmanTest.Podman([]string{"run", "busybox", "ls", "-l", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(18))
	})

	It("podman privileged should inherit host devices", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", ALPINE, "ls", "-l", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 20))
	})
})
