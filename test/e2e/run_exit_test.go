//go:build linux || freebsd

package integration

import (
	"fmt"

	"github.com/containers/podman/v5/libpod/define"
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run exit", func() {

	It("podman run exit define.ExecErrorCodeGeneric", func() {
		result := podmanTest.Podman([]string{"run", "--foobar", ALPINE, "ls", "$tmp"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(define.ExecErrorCodeGeneric, "unknown flag: --foobar"))
	})

	It("podman run exit ExecErrorCodeCannotInvoke", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "/etc"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(define.ExecErrorCodeCannotInvoke, "open executable: Operation not permitted: OCI permission denied"))
	})

	It("podman run exit ExecErrorCodeNotFound", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "foobar"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(define.ExecErrorCodeNotFound, "executable file `foobar` not found in $PATH: No such file or directory: OCI runtime attempted to invoke a command that was not found"))
	})

	It("podman run exit 0", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman run exit 50", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "sh", "-c", "exit 50"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(50, ""))
	})

	It("podman run exit 125", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "sh", "-c", fmt.Sprintf("exit %d", define.ExecErrorCodeGeneric)})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(define.ExecErrorCodeGeneric, ""))
	})
})
