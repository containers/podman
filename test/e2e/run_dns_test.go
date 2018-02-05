package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run dns", func() {
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

	It("podman run add search domain", func() {
		session := podmanTest.Podman([]string{"run", "--dns-search=foobar.com", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session.LineInOuputStartsWith("search foobar.com")
	})

	It("podman run add bad dns server", func() {
		session := podmanTest.Podman([]string{"run", "--dns=foobar", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman run add dns server", func() {
		session := podmanTest.Podman([]string{"run", "--dns=1.2.3.4", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session.LineInOuputStartsWith("server 1.2.3.4")
	})

	It("podman run add dns option", func() {
		session := podmanTest.Podman([]string{"run", "--dns-opt=debug", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session.LineInOuputStartsWith("options debug")
	})

	It("podman run add bad host", func() {
		session := podmanTest.Podman([]string{"run", "--add-host=foo:1.2", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman run add host", func() {
		session := podmanTest.Podman([]string{"run", "--add-host=foobar:1.1.1.1", ALPINE, "cat", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session.LineInOuputStartsWith("foobar 1.1.1.1")
	})

	It("podman run add hostname", func() {
		session := podmanTest.Podman([]string{"run", "--hostname=foobar", ALPINE, "cat", "/etc/hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("foobar"))

		session = podmanTest.Podman([]string{"run", "--hostname=foobar", ALPINE, "hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("foobar"))
	})
})
