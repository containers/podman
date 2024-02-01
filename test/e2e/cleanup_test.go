package integration

import (
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman container cleanup", func() {

	BeforeEach(func() {
		SkipIfRemote("podman container cleanup is not supported in remote")
	})

	It("podman cleanup bogus container", func() {
		session := podmanTest.Podman([]string{"container", "cleanup", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("no such container"))
	})

	It("podman cleanup container by id", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"container", "cleanup", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(cid))
	})

	It("podman cleanup container by short id", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		shortID := cid[0:10]
		session = podmanTest.Podman([]string{"container", "cleanup", shortID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(shortID))
	})

	It("podman cleanup container by name", func() {
		session := podmanTest.Podman([]string{"create", "--name", "foo", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"container", "cleanup", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("foo"))
	})

	It("podman cleanup all containers", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"container", "cleanup", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(cid))
	})

	It("podman cleanup latest container", func() {
		SkipIfRemote("--latest flag n/a")
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"container", "cleanup", "--latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(cid))
	})

	It("podman cleanup running container", func() {
		session := podmanTest.RunTopContainer("running")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"container", "cleanup", "running"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("container state improper"))
	})

	It("podman cleanup paused container", func() {
		SkipIfRootlessCgroupsV1("Pause is not supported in cgroups v1")
		session := podmanTest.RunTopContainer("paused")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"pause", "paused"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"container", "cleanup", "paused"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("container state improper"))

		// unpause so that the cleanup can stop the container,
		// otherwise it fails with container state improper
		session = podmanTest.Podman([]string{"unpause", "paused"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})
})
