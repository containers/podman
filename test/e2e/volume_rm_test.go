//go:build linux || freebsd

package integration

import (
	"fmt"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman volume rm", func() {

	AfterEach(func() {
		podmanTest.CleanupVolume()
	})

	It("podman volume rm", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "rm", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(BeEmpty())
	})

	It("podman volume rm with --force flag", func() {
		session := podmanTest.Podman([]string{"create", "-v", "myvol:/myvol", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "rm", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(2, fmt.Sprintf("volume myvol is being used by the following container(s): %s: volume is being used", cid)))

		session = podmanTest.Podman([]string{"volume", "rm", "-t", "0", "-f", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(BeEmpty())
	})

	It("podman volume remove bogus", func() {
		session := podmanTest.Podman([]string{"volume", "rm", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, `no volume with name "bogus" found: no such volume`))
	})

	It("podman rm with --all flag", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "rm", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(BeEmpty())
	})

	It("podman volume rm by partial name", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "rm", "myv"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(BeEmpty())
	})

	It("podman volume rm by nonunique partial name", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", "myvol2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "rm", "myv"})
		session.WaitWithDefaultTimeout()
		expect := "more than one result for volume name myv: volume already exists"
		if podmanTest.DatabaseBackend == "boltdb" {
			// boltdb issues volume name in quotes
			expect = `more than one result for volume name "myv": volume already exists`
		}
		Expect(session).To(ExitWithError(125, expect))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">=", 2))
	})
})
