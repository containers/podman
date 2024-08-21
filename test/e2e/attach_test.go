//go:build linux || freebsd

package integration

import (
	"syscall"
	"time"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman attach", func() {

	It("podman attach to bogus container", func() {
		session := podmanTest.Podman([]string{"attach", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `no container with name or ID "foobar" found: no such container`))
	})

	It("podman attach to non-running container", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test1", "-i", CITEST_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"attach", "test1"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitWithError(125, "you can only attach to running containers"))
	})

	It("podman container attach to non-running container", func() {
		session := podmanTest.Podman([]string{"container", "create", "--name", "test1", "-i", CITEST_IMAGE, "ls"})

		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"container", "attach", "test1"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitWithError(125, "you can only attach to running containers"))
	})

	It("podman attach to multiple containers", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainer("test2")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"attach", "test1", "test2"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitWithError(125, " attach` accepts at most one argument"))
	})

	It("podman attach to a running container", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test", CITEST_IMAGE, "/bin/sh", "-c", "while true; do echo test; sleep 1; done"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"attach", "test"})
		time.Sleep(2 * time.Second)
		results.Signal(syscall.SIGTSTP)
		Expect(results.OutputToString()).To(ContainSubstring("test"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman attach to the latest container", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test1", CITEST_IMAGE, "/bin/sh", "-c", "while true; do echo test1; sleep 1; done"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "-d", "--name", "test2", CITEST_IMAGE, "/bin/sh", "-c", "while true; do echo test2; sleep 1; done"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

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
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test", CITEST_IMAGE, "/bin/sh", "-c", "while true; do echo test; sleep 1; done"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		results := podmanTest.Podman([]string{"attach", "--sig-proxy=false", "test"})
		time.Sleep(2 * time.Second)
		results.Signal(syscall.SIGTERM)
		results.WaitWithDefaultTimeout()
		Expect(results.OutputToString()).To(ContainSubstring("test"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})
})
