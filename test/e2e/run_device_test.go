package integration

import (
	"fmt"
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run device", func() {
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
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

	It("podman run device rename test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--device", "/dev/kmsg:/dev/kmsg1", ALPINE, "ls", "--color=never", "/dev/kmsg1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("/dev/kmsg1"))
	})

	It("podman run device permission test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--device", "/dev/kmsg:r", ALPINE, "ls", "--color=never", "/dev/kmsg"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("/dev/kmsg"))
	})

	It("podman run device rename and permission test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--device", "/dev/kmsg:/dev/kmsg1:r", ALPINE, "ls", "--color=never", "/dev/kmsg1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("/dev/kmsg1"))
	})
	It("podman run device rename and bad permission test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--device", "/dev/kmsg:/dev/kmsg1:rd", ALPINE, "ls", "--color=never", "/dev/kmsg1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})
})
