//go:build linux || freebsd

package integration

import (
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman wait", func() {

	It("podman wait on bogus container", func() {
		session := podmanTest.Podman([]string{"wait", "1234"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `no container with name or ID "1234" found: no such container`))

	})

	It("podman wait on a stopped container", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "ls"})
		session.Wait(10)
		cid := session.OutputToString()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"wait", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman wait on a sleeping container", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		cid := session.OutputToString()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"wait", cid})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
	})

	It("podman wait on latest container", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		if IsRemote() {
			session = podmanTest.Podman([]string{"wait", session.OutputToString()})
		} else {
			session = podmanTest.Podman([]string{"wait", "-l"})
		}
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman container wait on latest container", func() {
		session := podmanTest.Podman([]string{"container", "run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		if IsRemote() {
			session = podmanTest.Podman([]string{"container", "wait", session.OutputToString()})
		} else {
			session = podmanTest.Podman([]string{"container", "wait", "-l"})
		}
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman container wait on latest container with --interval flag", func() {
		session := podmanTest.Podman([]string{"container", "run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"container", "wait", "-i", "5000", session.OutputToString()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman container wait on latest container with --interval flag", func() {
		session := podmanTest.Podman([]string{"container", "run", "-d", ALPINE, "sleep", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"container", "wait", "--interval", "1s", session.OutputToString()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman container wait on container with bogus --interval", func() {
		session := podmanTest.Podman([]string{"container", "run", "-d", ALPINE, "sleep", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"container", "wait", "--interval", "100days", session.OutputToString()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `time: unknown unit "days" in duration "100days"`))
	})

	It("podman wait on three containers", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		cid1 := session.OutputToString()
		session = podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		cid2 := session.OutputToString()
		session = podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "1"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		cid3 := session.OutputToString()
		session = podmanTest.Podman([]string{"wait", cid1, cid2, cid3})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(Equal([]string{"0", "0", "0"}))
	})

	It("podman wait on multiple conditions", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "100"})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		// condition should return once nay of the condition is met not all of them,
		// as the container is running this should return immediately
		// https://github.com/containers/podman-py/issues/425
		session = podmanTest.Podman([]string{"wait", "--condition", "running,exited", cid})
		session.Wait(20)
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("-1"))
	})
})
