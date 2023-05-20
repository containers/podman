package integration

import (
	"fmt"

	"github.com/containers/podman/v4/libpod/define"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run exit", func() {

	It("podman run exit define.ExecErrorCodeGeneric", func() {
		result := podmanTest.Podman([]string{"run", "--foobar", ALPINE, "ls", "$tmp"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(define.ExecErrorCodeGeneric))
	})

	It("podman run exit ExecErrorCodeCannotInvoke", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "/etc"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(define.ExecErrorCodeCannotInvoke))
	})

	It("podman run exit ExecErrorCodeNotFound", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "foobar"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(define.ExecErrorCodeNotFound))
	})

	It("podman run exit 0", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman run exit 50", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "sh", "-c", "exit 50"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(50))
	})

	It("podman run exit 125", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "sh", "-c", fmt.Sprintf("exit %d", define.ExecErrorCodeGeneric)})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(define.ExecErrorCodeGeneric))
	})
})
