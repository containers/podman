package integration

import (
	"os"

	"fmt"
	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run passwd", func() {
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

	It("podman run no user specified ", func() {
		session := podmanTest.Podman([]string{"run", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("passwd")).To(BeFalse())
	})
	It("podman run user specified in container", func() {
		session := podmanTest.Podman([]string{"run", "-u", "bin", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("passwd")).To(BeFalse())
	})

	It("podman run UID specified in container", func() {
		session := podmanTest.Podman([]string{"run", "-u", "2:1", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("passwd")).To(BeFalse())
	})

	It("podman run UID not specified in container", func() {
		session := podmanTest.Podman([]string{"run", "-u", "20001:1", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("passwd")).To(BeTrue())
	})
})
