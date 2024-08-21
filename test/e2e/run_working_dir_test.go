//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run", func() {

	It("podman run a container without workdir", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("/"))
	})

	It("podman run a container using non existing --workdir", func() {
		session := podmanTest.Podman([]string{"run", "--workdir", "/home/foobar", ALPINE, "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(126, `workdir "/home/foobar" does not exist on container `))
	})

	It("podman run a container using a --workdir under a bind mount", func() {
		volume := filepath.Join(podmanTest.TempDir, "vol")
		err = os.MkdirAll(volume, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "--volume", fmt.Sprintf("%s:/var_ovl/:O", volume), "--workdir", "/var_ovl/log", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run a container on an image with a workdir", func() {
		dockerfile := fmt.Sprintf(`FROM %s
RUN  mkdir -p /home/foobar /etc/foobar; chown bin:bin /etc/foobar
WORKDIR  /etc/foobar`, ALPINE)
		podmanTest.BuildImage(dockerfile, "test", "false")

		session := podmanTest.Podman([]string{"run", "test", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("/etc/foobar"))

		session = podmanTest.Podman([]string{"run", "test", "ls", "-ld", "."})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("bin"))

		session = podmanTest.Podman([]string{"run", "--workdir", "/home/foobar", "test", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("/home/foobar"))
	})
})
