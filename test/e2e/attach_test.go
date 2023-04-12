package integration

import (
	"syscall"
	"time"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman attach", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
	})

	It("podman attach to bogus container", func() {
		session := podmanTest.Podman([]string{"attach", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman attach to non-running container", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test1", "-i", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		results := podmanTest.Podman([]string{"attach", "test1"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(125))
	})

	It("podman container attach to non-running container", func() {
		session := podmanTest.Podman([]string{"container", "create", "--name", "test1", "-i", ALPINE, "ls"})

		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		results := podmanTest.Podman([]string{"container", "attach", "test1"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(125))
	})

	It("podman attach to multiple containers", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.RunTopContainer("test2")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		results := podmanTest.Podman([]string{"attach", "test1", "test2"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(125))
	})

	It("podman attach to a running container", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test", ALPINE, "/bin/sh", "-c", "while true; do echo test; sleep 1; done"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		results := podmanTest.Podman([]string{"attach", "test"})
		time.Sleep(2 * time.Second)
		results.Signal(syscall.SIGTSTP)
		Expect(results.OutputToString()).To(ContainSubstring("test"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman attach to the latest container", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test1", ALPINE, "/bin/sh", "-c", "while true; do echo test1; sleep 1; done"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "-d", "--name", "test2", ALPINE, "/bin/sh", "-c", "while true; do echo test2; sleep 1; done"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		cid := "-l"
		if IsRemote() {
			cid = "test2"
		}
		results := podmanTest.Podman([]string{"attach", cid})
		time.Sleep(2 * time.Second)
		results.Signal(syscall.SIGTSTP)
		Expect(results.OutputToString()).To(ContainSubstring("test2"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})

	It("podman attach to a container with --sig-proxy set to false", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test", ALPINE, "/bin/sh", "-c", "while true; do echo test; sleep 1; done"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		results := podmanTest.Podman([]string{"attach", "--sig-proxy=false", "test"})
		time.Sleep(2 * time.Second)
		results.Signal(syscall.SIGTERM)
		results.WaitWithDefaultTimeout()
		Expect(results.OutputToString()).To(ContainSubstring("test"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})
})
