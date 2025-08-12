//go:build linux || freebsd

package integration

import (
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman volume prune", func() {

	AfterEach(func() {
		podmanTest.CleanupVolume()
	})

	It("podman prune volume", func() {
		session := podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "-v", "myvol:/myvol", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(4))

		session = podmanTest.Podman([]string{"volume", "prune", "--force"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman prune volume --filter until", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "label1=value1", "myvol1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))

		session = podmanTest.Podman([]string{"volume", "prune", "--force", "--filter", "until=50"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))

		session = podmanTest.Podman([]string{"volume", "prune", "--force", "--filter", "until=5000000000"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman prune volume --filter", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "label1=value1", "myvol1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", "--label", "sharedlabel1=slv1", "myvol2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", "--label", "sharedlabel1=slv2", "myvol3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", "--label", "sharedlabel1", "myvol4"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "-v", "myvol5:/myvol5", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "-v", "myvol6:/myvol6", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(7))

		session = podmanTest.Podman([]string{"volume", "prune", "--force", "--filter", "label=label1=value1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(6))

		session = podmanTest.Podman([]string{"volume", "prune", "--force", "--filter", "label=sharedlabel1=slv1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(5))

		session = podmanTest.Podman([]string{"volume", "prune", "--force", "--filter", "label=sharedlabel1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(3))

		session = podmanTest.Podman([]string{"volume", "create", "--label", "testlabel", "myvol7"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "prune", "--force", "--filter", "label!=testlabel"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman system prune --volume", func() {
		useCustomNetworkDir(podmanTest, tempdir)
		session := podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "-v", "myvol:/myvol", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(4))

		session = podmanTest.Podman([]string{"system", "prune", "--force", "--volumes"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman volume prune --filter since/after", func() {
		vol1 := "vol1"
		vol2 := "vol2"
		vol3 := "vol3"

		session := podmanTest.Podman([]string{"volume", "create", vol1})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", vol2})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", vol3})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "prune", "-f", "--filter", "since=" + vol1})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToStringArray()).To(HaveLen(1))
		Expect(session.OutputToStringArray()[0]).To(Equal(vol1))
	})

	It("podman volume prune filters should combine with AND logic", func() {
		// Create volumes with different label combinations to test AND logic
		session := podmanTest.Podman([]string{"volume", "create", "--label", "a=b", "prune-vol-with-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volWithA := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "create", "--label", "c=d", "prune-vol-with-c"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volWithC := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "create", "--label", "a=b", "--label", "c=d", "prune-vol-with-both"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volWithBoth := session.OutputToString()

		// Verify all volumes exist before pruning
		session = podmanTest.Podman([]string{"volume", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(3)) // 3 created volumes

		// Test AND logic: only volumes with both label=a=b AND label=c=d should be pruned
		session = podmanTest.Podman([]string{"volume", "prune", "-f", "--filter", "label=a=b", "--filter", "label=c=d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Check remaining volumes - only volWithBoth should be pruned
		session = podmanTest.Podman([]string{"volume", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		remaining := session.OutputToStringArray()
		Expect(remaining).To(HaveLen(2)) // 2 remaining volumes
		Expect(remaining).To(ContainElement(volWithA))
		Expect(remaining).To(ContainElement(volWithC))
		Expect(remaining).NotTo(ContainElement(volWithBoth))

		// Clean up for next test
		session = podmanTest.Podman([]string{"volume", "prune", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Create new volumes for next test
		session = podmanTest.Podman([]string{"volume", "create", "--label", "a=b", "prune-vol-with-a2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volWithA2 := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "create", "--label", "a=b", "--label", "c=d", "prune-vol-with-both2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volWithBoth2 := session.OutputToString()

		// Test AND logic: label=a=b AND label!=c=d should only prune volumes with a=b but without c=d
		session = podmanTest.Podman([]string{"volume", "prune", "-f", "--filter", "label=a=b", "--filter", "label!=c=d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Check remaining volumes - only volWithA2 should be pruned, volWithBoth2 should remain
		session = podmanTest.Podman([]string{"volume", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		remaining = session.OutputToStringArray()
		Expect(remaining).To(HaveLen(1)) // 1 remaining volume
		Expect(remaining).To(ContainElement(volWithBoth2))
		Expect(remaining).NotTo(ContainElement(volWithA2))
	})

})
