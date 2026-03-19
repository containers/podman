//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman volume prune", func() {
	AfterEach(func() {
		podmanTest.CleanupVolume()
	})

	It("podman prune volume -a removes all unused volumes", func() {
		podmanTest.PodmanExitCleanly("volume", "create")
		podmanTest.PodmanExitCleanly("volume", "create")
		podmanTest.PodmanExitCleanly("create", "-v", "myvol:/myvol", ALPINE, "ls")

		session := podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(4))

		podmanTest.PodmanExitCleanly("volume", "prune", "-a", "--force")

		session = podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman volume prune", func() {
		session := podmanTest.PodmanExitCleanly("create", "-v", "/anon", ALPINE, "top")
		podmanTest.PodmanExitCleanly("rm", session.OutputToString())

		podmanTest.PodmanExitCleanly("volume", "create", "named_vol")

		session = podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(3))

		podmanTest.PodmanExitCleanly("volume", "prune", "--force")

		session = podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToString()).To(ContainSubstring("named_vol"))
	})

	It("podman volume prune --filter all=true removes all unused volumes", func() {
		podmanTest.PodmanExitCleanly("volume", "create", "prune_filter_all_test")
		podmanTest.PodmanExitCleanly("volume", "prune", "--filter", "all=true", "--force")

		session := podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman prune volume --filter until", func() {
		podmanTest.PodmanExitCleanly("volume", "create", "--label", "label1=value1", "myvol1")

		session := podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(2))

		podmanTest.PodmanExitCleanly("volume", "prune", "--force", "--filter", "until=50")

		session = podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(2))

		podmanTest.PodmanExitCleanly("volume", "prune", "--force", "--filter", "until=5000000000")

		session = podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman prune volume --filter", func() {
		podmanTest.PodmanExitCleanly("volume", "create", "--label", "label1=value1", "myvol1")
		podmanTest.PodmanExitCleanly("volume", "create", "--label", "sharedlabel1=slv1", "myvol2")
		podmanTest.PodmanExitCleanly("volume", "create", "--label", "sharedlabel1=slv2", "myvol3")
		podmanTest.PodmanExitCleanly("volume", "create", "--label", "sharedlabel1", "myvol4")
		podmanTest.PodmanExitCleanly("create", "-v", "myvol5:/myvol5", ALPINE, "ls")
		podmanTest.PodmanExitCleanly("create", "-v", "myvol6:/myvol6", ALPINE, "ls")

		session := podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(7))

		podmanTest.PodmanExitCleanly("volume", "prune", "--force", "--filter", "label=label1=value1")

		session = podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(6))

		podmanTest.PodmanExitCleanly("volume", "prune", "--force", "--filter", "label=sharedlabel1=slv1")

		session = podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(5))

		podmanTest.PodmanExitCleanly("volume", "prune", "--force", "--filter", "label=sharedlabel1")

		session = podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(3))

		podmanTest.PodmanExitCleanly("volume", "create", "--label", "testlabel", "myvol7")
		podmanTest.PodmanExitCleanly("volume", "prune", "--force", "--filter", "label!=testlabel")
	})

	It("podman system prune --volume", func() {
		useCustomNetworkDir(podmanTest, tempdir)
		podmanTest.PodmanExitCleanly("volume", "create")
		podmanTest.PodmanExitCleanly("volume", "create")
		podmanTest.PodmanExitCleanly("create", "-v", "myvol:/myvol", ALPINE, "ls")

		session := podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(4))

		podmanTest.PodmanExitCleanly("system", "prune", "--force", "--volumes")

		session = podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman volume prune --filter since/after", func() {
		vol1 := "vol1"
		vol2 := "vol2"
		vol3 := "vol3"

		podmanTest.PodmanExitCleanly("volume", "create", vol1)
		podmanTest.PodmanExitCleanly("volume", "create", vol2)
		podmanTest.PodmanExitCleanly("volume", "create", vol3)

		podmanTest.PodmanExitCleanly("volume", "prune", "-f", "--filter", "since="+vol1)

		session := podmanTest.PodmanExitCleanly("volume", "ls", "-q")
		Expect(session.OutputToStringArray()).To(HaveLen(1))
		Expect(session.OutputToStringArray()[0]).To(Equal(vol1))
	})
})
