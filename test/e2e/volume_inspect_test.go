//go:build linux || freebsd

package integration

import (
	"github.com/containers/podman/v5/libpod/define"
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman volume inspect", func() {

	AfterEach(func() {
		podmanTest.CleanupVolume()
	})

	It("podman inspect volume", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "inspect", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(BeValidJSON())
	})

	It("podman inspect volume with Go format", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "inspect", "--format", "{{.Name}}", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(volName))
	})

	It("podman inspect volume with --all flag", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol1"})
		session.WaitWithDefaultTimeout()
		volName1 := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", "myvol2"})
		session.WaitWithDefaultTimeout()
		volName2 := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "inspect", "--format", "{{.Name}}", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()[0]).To(Equal(volName1))
		Expect(session.OutputToStringArray()[1]).To(Equal(volName2))
	})

	It("inspect volume finds options", func() {
		volName := "testvol"
		session := podmanTest.Podman([]string{"volume", "create", "--opt", "type=tmpfs", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"volume", "inspect", volName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(define.TypeTmpfs))
	})
})
