package integration

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run entrypoint", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tempdir)
		podmanTest.RestoreArtifact(ALPINE)
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman run no command, entrypoint, or cmd", func() {
		dockerfile := `FROM docker.io/library/alpine:latest
ENTRYPOINT []
CMD []
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman run entrypoint", func() {
		dockerfile := `FROM docker.io/library/alpine:latest
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(2))
	})

	It("podman run entrypoint with cmd", func() {
		dockerfile := `FROM docker.io/library/alpine:latest
CMD [ "-v"]
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(4))
	})

	It("podman run entrypoint with user cmd overrides image cmd", func() {
		dockerfile := `FROM docker.io/library/alpine:latest
CMD [ "-v"]
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest", "-i"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(5))
	})

	It("podman run entrypoint with user cmd no image cmd", func() {
		dockerfile := `FROM docker.io/library/alpine:latest
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest", "-i"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(5))
	})

	It("podman run user entrypoint overrides image entrypoint and image cmd", func() {
		dockerfile := `FROM docker.io/library/alpine:latest
CMD ["-i"]
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "--entrypoint=uname", "foobar.com/entrypoint:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOuputStartsWith("Linux")).To(BeTrue())
	})

	It("podman run user entrypoint with command overrides image entrypoint and image cmd", func() {
		dockerfile := `FROM docker.io/library/alpine:latest
CMD ["-i"]
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "--entrypoint=uname", "foobar.com/entrypoint:latest", "-r"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOuputStartsWith("Linux")).To(BeFalse())
	})
})
