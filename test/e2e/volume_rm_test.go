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
		expected := "Error: no such volume"
		if podmanTest.DatabaseBackend == "boltdb" {
			expected = "Error: volume with name bogus not found: no such volume"
		}
		Expect(session).Should(ExitWithError(1, expected))
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

	It("podman volume remove require exact name", func() {
		session := podmanTest.Podman([]string{"volume", "create", defaultVolName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		nameStart := defaultVolName[:3]
		session = podmanTest.Podman([]string{"volume", "rm", nameStart})
		session.WaitWithDefaultTimeout()
		expectedRm := "Error: no such volume"
		if podmanTest.DatabaseBackend == "boltdb" {
			expectedRm = fmt.Sprintf("Error: volume with name %s not found: no such volume", nameStart)
		}
		Expect(session).Should(ExitWithError(1, expectedRm))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToStringArray()
		expectedLs := []string{"DRIVER      VOLUME NAME", fmt.Sprintf("local       %s", defaultVolName)}
		Expect(output).To(Equal(expectedLs))
	})
})
