//go:build linux || freebsd

package integration

import (
	"os"
	"path/filepath"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman quadlet verb", func() {

	BeforeEach(func() {
		SkipIfRemote("`podman quadlet` is not yet implemented for remote")
		SkipIfSystemdNotRunning("cannot test systemd is not running")
	})

	It("podman quadlet install, list, rm", func() {

		quadletfileContent, err := os.ReadFile(filepath.Join("quadlet", "alpine-quadlet.container"))
		Expect(err).ToNot(HaveOccurred())

		result := podmanTest.PodmanExitCleanly("quadlet", "install", filepath.Join("quadlet", "alpine-quadlet.container"))
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		// List should contain alpine-quadlet
		listSession := podmanTest.PodmanExitCleanly("quadlet", "list")
		Expect(listSession).Should(ExitCleanly())
		Expect(listSession.OutputToString()).To(ContainSubstring("alpine-quadlet.container"))

		// List should not contain `alpine-quadlet.container`
		listSession = podmanTest.PodmanExitCleanly("quadlet", "list", "--filter", "name=something*")
		Expect(listSession).Should(ExitCleanly())
		Expect(listSession.OutputToString()).To(Not(ContainSubstring("alpine-quadlet.container")))

		// List should contain `alpine-quadlet.container`
		listSession = podmanTest.PodmanExitCleanly("quadlet", "list", "--filter", "name=alpine*")
		Expect(listSession).Should(ExitCleanly())
		Expect(listSession.OutputToString()).To(ContainSubstring("alpine-quadlet.container"))

		printSession := podmanTest.PodmanExitCleanly("quadlet", "print", "alpine-quadlet.container")
		Expect(printSession).Should(ExitCleanly())
		Expect(string(printSession.Out.Contents())).To(Equal(string(quadletfileContent)))

		// Remove quadlet container
		result = podmanTest.PodmanExitCleanly("quadlet", "rm", "alpine-quadlet.container")
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		// Try to list again and output should be empty
		listSession = podmanTest.PodmanExitCleanly("quadlet", "list")
		Expect(listSession).Should(ExitCleanly())
		Expect(listSession.OutputToString()).To(Not(ContainSubstring("alpine-quadlet.container")))
	})
})
