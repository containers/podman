//go:build linux || freebsd

package integration

import (
	"fmt"
	"path"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Create an image with the given seccomp label
func makeLabeledImage(seccompLabel string) string {
	ctrName := "temp-working-container"
	imgName := "workingimage"
	session := podmanTest.Podman([]string{"run", "--name", ctrName, CITEST_IMAGE, "true"})
	session.WaitWithDefaultTimeout()
	Expect(session).To(ExitCleanly())

	session = podmanTest.Podman([]string{"commit", "-q", "--change", "LABEL io.containers.seccomp.profile=" + seccompLabel, ctrName, imgName})
	session.WaitWithDefaultTimeout()
	Expect(session).To(ExitCleanly())

	return imgName
}

var _ = Describe("Podman run", func() {

	It("podman run --seccomp-policy default", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--seccomp-policy", "default", CITEST_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run --seccomp-policy ''", func() {
		// Empty string is interpreted as "default".
		session := podmanTest.Podman([]string{"run", "-q", "--seccomp-policy", "", CITEST_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run --seccomp-policy invalid", func() {
		session := podmanTest.Podman([]string{"run", "--seccomp-policy", "invalid", CITEST_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `invalid seccomp policy "invalid": valid policies are ["default" "image"]`))
	})

	It("podman run --seccomp-policy image (block all syscalls)", func() {
		// This image has seccomp profiles that blocks all syscalls.
		// The intention behind blocking all syscalls is to prevent
		// regressions in the future.  The required syscalls can vary
		// depending on which runtime we're using.
		img := makeLabeledImage(`'{"defaultAction":"SCMP_ACT_ERRNO"}'`)
		session := podmanTest.Podman([]string{"run", "--seccomp-policy", "image", img, "ls"})
		session.WaitWithDefaultTimeout()

		switch path.Base(podmanTest.OCIRuntime) {
		case "crun":
			expect := fmt.Sprintf("OCI runtime error: %s: read from the init process", podmanTest.OCIRuntime)
			if IsRemote() {
				expect = fmt.Sprintf("for attach: %s: read from the init process: OCI runtime error", podmanTest.OCIRuntime)
			}
			Expect(session).To(ExitWithError(126, expect))
		case "runc":
			expect1 := "cannot start a container that has stopped"
			c1 := 126
			expect2 := "failed to connect to container's attach socket"
			c2 := 127
			Expect(session).To(Or(ExitWithError(c1, expect1), ExitWithError(c2, expect2)))
		default:
			Expect(session.ExitCode()).To(BeNumerically(">", 0), "Exit status using generic runtime")
		}
	})

	It("podman run --seccomp-policy image (bogus profile)", func() {
		// This image has a bogus/invalid seccomp profile which should
		// yield a json error when being read.
		img := makeLabeledImage(`'BOGUS - this should yield an error'`)
		session := podmanTest.Podman([]string{"run", "--seccomp-policy", "image", img, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "loading seccomp profile failed: decoding seccomp profile failed: invalid character 'B' looking for beginning of value"))
	})
})
