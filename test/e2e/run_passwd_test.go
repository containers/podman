// +build !remoteclient

package integration

import (
	"os"

	. "github.com/containers/libpod/v2/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run passwd", func() {
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

	It("podman run no user specified ", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("passwd")).To(BeFalse())
	})
	It("podman run user specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "bin", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("passwd")).To(BeFalse())
	})

	It("podman run UID specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "2:1", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("passwd")).To(BeFalse())
	})

	It("podman run UID not specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "20001:1", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("passwd")).To(BeTrue())
	})

	It("podman can run container without /etc/passwd", func() {
		SkipIfRemote()
		dockerfile := `FROM alpine
RUN rm -f /etc/passwd /etc/shadow /etc/group
USER 1000`
		imgName := "testimg"
		podmanTest.BuildImage(dockerfile, imgName, "false")
		session := podmanTest.Podman([]string{"run", "--rm", imgName, "ls", "/etc/"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("passwd")))
	})
})
