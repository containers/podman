package integration

import (
	"os"
	"strings"

	. "github.com/containers/podman/v2/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman run a container without workdir", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("/"))
	})

	It("podman run a container using non existing --workdir", func() {
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test only works on crun")
		}
		session := podmanTest.Podman([]string{"run", "--workdir", "/home/foobar", ALPINE, "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(127))
	})

	It("podman run a container on an image with a workdir", func() {
		SkipIfRemote("FIXME This should work on podman-remote")
		dockerfile := `FROM alpine
RUN  mkdir -p /home/foobar
WORKDIR  /etc/foobar`
		podmanTest.BuildImage(dockerfile, "test", "false")

		session := podmanTest.Podman([]string{"run", "test", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("/etc/foobar"))

		session = podmanTest.Podman([]string{"run", "--workdir", "/home/foobar", "test", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("/home/foobar"))
	})
})
