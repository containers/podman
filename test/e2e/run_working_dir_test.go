package integration

import (
	"fmt"
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman run a container without workdir", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("/"))
	})

	It("podman run a container using non existing --workdir", func() {
		session := podmanTest.Podman([]string{"run", "--workdir", "/home/foobar", ALPINE, "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(126))
	})

	It("podman run a container using a --workdir under a bind mount", func() {
		volume, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "--volume", fmt.Sprintf("%s:/var_ovl/:O", volume), "--workdir", "/var_ovl/log", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run a container on an image with a workdir", func() {
		dockerfile := fmt.Sprintf(`FROM %s
RUN  mkdir -p /home/foobar /etc/foobar; chown bin:bin /etc/foobar
WORKDIR  /etc/foobar`, ALPINE)
		podmanTest.BuildImage(dockerfile, "test", "false")

		session := podmanTest.Podman([]string{"run", "test", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("/etc/foobar"))

		session = podmanTest.Podman([]string{"run", "test", "ls", "-ld", "."})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("bin"))

		session = podmanTest.Podman([]string{"run", "--workdir", "/home/foobar", "test", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("/home/foobar"))
	})
})
