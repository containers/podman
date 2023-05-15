package integration

import (
	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run restart containers", func() {

	It("Podman start after successful run", func() {
		session := podmanTest.Podman([]string{"run", "--name", "test", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"wait", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session2 := podmanTest.Podman([]string{"start", "--attach", "test"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
	})

	It("Podman start after signal kill", func() {
		_ = podmanTest.RunTopContainer("test1")
		ok := WaitForContainer(podmanTest)
		Expect(ok).To(BeTrue(), "test1 container started")

		killSession := podmanTest.Podman([]string{"kill", "-s", "9", "test1"})
		killSession.WaitWithDefaultTimeout()
		Expect(killSession).Should(Exit(0))

		session2 := podmanTest.Podman([]string{"start", "test1"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
	})
})
