package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

// system reset must run serial: https://github.com/containers/podman/issues/17903
var _ = Describe("podman system reset", Serial, func() {

	It("podman system reset", func() {
		SkipIfRemote("system reset not supported on podman --remote")
		// system reset will not remove additional store images, so need to grab length
		useCustomNetworkDir(podmanTest, tempdir)

		session := podmanTest.Podman([]string{"rmi", "--force", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"images", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		l := len(session.OutputToStringArray())

		podmanTest.AddImageToRWStore(ALPINE)
		session = podmanTest.Podman([]string{"volume", "create", "data"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "-v", "data:/data", ALPINE, "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"system", "reset", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		Expect(session.ErrorToString()).To(Not(ContainSubstring("Failed to add pause process")))
		Expect(session.ErrorToString()).To(Not(ContainSubstring("/usr/share/containers/storage.conf")))

		session = podmanTest.Podman([]string{"images", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(l))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())

		session = podmanTest.Podman([]string{"container", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())

		session = podmanTest.Podman([]string{"network", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// default network should exists
		Expect(session.OutputToStringArray()).To(HaveLen(1))

		// TODO: machine tests currently don't run outside of the machine test pkg
		// no machines are created here to cleanup
		// machine commands are rootless only
		if isRootless() {
			session = podmanTest.Podman([]string{"machine", "list", "-q"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.OutputToStringArray()).To(BeEmpty())
		}
	})
})
