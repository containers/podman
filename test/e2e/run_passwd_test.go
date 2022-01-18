package integration

import (
	"fmt"
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("passwd")))
	})
	It("podman run user specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "bin", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("passwd")))
	})

	It("podman run UID specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "2:1", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("passwd")))
	})

	It("podman run UID not specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "20001:1", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("passwd"))
	})

	It("podman can run container without /etc/passwd", func() {
		dockerfile := fmt.Sprintf(`FROM %s
RUN rm -f /etc/passwd /etc/shadow /etc/group
USER 1000`, ALPINE)
		imgName := "testimg"
		podmanTest.BuildImage(dockerfile, imgName, "false")
		session := podmanTest.Podman([]string{"run", "--rm", imgName, "ls", "/etc/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("passwd")))
	})

	It("podman run with no user specified does not change --group specified", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("/etc/group")))
	})

	It("podman run group specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "root:bin", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("/etc/group")))
	})

	It("podman run non-numeric group not specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "root:doesnotexist", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run numeric group specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "root:11", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("/etc/group")))
	})

	It("podman run numeric group not specified in container", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "20001:20001", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("/etc/group"))
	})

	It("podman run numeric user not specified in container modifies group", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", "-u", "20001", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("/etc/group"))
	})

	It("podman run numeric group from image and no group file", func() {
		dockerfile := fmt.Sprintf(`FROM %s
RUN rm -f /etc/passwd /etc/shadow /etc/group
USER 1000`, ALPINE)
		imgName := "testimg"
		podmanTest.BuildImage(dockerfile, imgName, "false")
		session := podmanTest.Podman([]string{"run", "--rm", imgName, "ls", "/etc/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("/etc/group")))
	})

	It("podman run --no-manage-passwd flag", func() {
		run := podmanTest.Podman([]string{"run", "--user", "1234:1234", ALPINE, "cat", "/etc/passwd"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(run.OutputToString()).To(ContainSubstring("1234:1234"))

		run = podmanTest.Podman([]string{"run", "--passwd=false", "--user", "1234:1234", ALPINE, "cat", "/etc/passwd"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(run.OutputToString()).NotTo((ContainSubstring("1234:1234")))
	})
})
