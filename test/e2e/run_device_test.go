package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman kill", func() {
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()

	})

	It("podman run bad device test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--device", "/dev/baddevice", ALPINE, "ls", "/dev/kmsg"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman run device test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--device", "/dev/kmsg", ALPINE, "ls", "--color=never", "/dev/kmsg"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("/dev/kmsg"))
	})
})
