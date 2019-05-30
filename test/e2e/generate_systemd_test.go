// +build !remoteclient

package integration

import (
	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
)

var _ = Describe("Podman generate systemd", func() {
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

	It("podman generate systemd on bogus container", func() {
		session := podmanTest.Podman([]string{"generate", "systemd", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman generate systemd bad restart policy", func() {
		session := podmanTest.Podman([]string{"generate", "systemd", "--restart-policy", "never", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman generate systemd bad timeout value", func() {
		session := podmanTest.Podman([]string{"generate", "systemd", "--timeout", "-1", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman generate systemd", func() {
		n := podmanTest.Podman([]string{"run", "--name", "nginx", "-dt", nginx})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "nginx"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman generate systemd with timeout", func() {
		n := podmanTest.Podman([]string{"run", "--name", "nginx", "-dt", nginx})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "--timeout", "5", "nginx"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

})
