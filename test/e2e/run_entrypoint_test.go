package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run entrypoint", func() {
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
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman run no command, entrypoint, or cmd", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT []
CMD []
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman run entrypoint == [\"\"]", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT [""]
CMD []
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest", "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("hello"))
	})

	It("podman run entrypoint", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman run entrypoint with cmd", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
CMD [ "-v"]
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(4))
	})

	It("podman run entrypoint with user cmd overrides image cmd", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
CMD [ "-v"]
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest", "-i"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(5))
	})

	It("podman run entrypoint with user cmd no image cmd", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest", "-i"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(5))
	})

	It("podman run user entrypoint overrides image entrypoint and image cmd", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
CMD ["-i"]
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "--entrypoint=uname", "foobar.com/entrypoint:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("Linux")))

		session = podmanTest.Podman([]string{"run", "--entrypoint", "", "foobar.com/entrypoint:latest", "uname"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("Linux")))
	})

	It("podman run user entrypoint with command overrides image entrypoint and image cmd", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
CMD ["-i"]
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "--entrypoint=uname", "foobar.com/entrypoint:latest", "-r"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(Not(ContainElement(HavePrefix("Linux"))))
	})
})
