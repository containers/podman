package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run dns", func() {
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

	It("podman run add search domain", func() {
		session := podmanTest.Podman([]string{"run", "--dns-search=foobar.com", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("search foobar.com")))
	})

	It("podman run remove all search domain", func() {
		session := podmanTest.Podman([]string{"run", "--dns-search=.", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(Not(ContainElement(HavePrefix("search"))))
	})

	It("podman run add bad dns server", func() {
		session := podmanTest.Podman([]string{"run", "--dns=foobar", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run add dns server", func() {
		session := podmanTest.Podman([]string{"run", "--dns=1.2.3.4", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("nameserver 1.2.3.4")))

	})

	It("podman run add dns option", func() {
		session := podmanTest.Podman([]string{"run", "--dns-opt=debug", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("options debug")))
	})

	It("podman run add bad host", func() {
		session := podmanTest.Podman([]string{"run", "--add-host=foo:1.2", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run add host", func() {
		session := podmanTest.Podman([]string{"run", "--add-host=foobar:1.1.1.1", "--add-host=foobaz:2001:db8::68", ALPINE, "cat", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("1.1.1.1 foobar")))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("2001:db8::68 foobaz")))
	})

	It("podman run add hostname", func() {
		session := podmanTest.Podman([]string{"run", "--hostname=foobar", ALPINE, "cat", "/etc/hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("foobar"))

		session = podmanTest.Podman([]string{"run", "--hostname=foobar", ALPINE, "hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("foobar"))
	})

	It("podman run add hostname sets /etc/hosts", func() {
		session := podmanTest.Podman([]string{"run", "-t", "-i", "--hostname=foobar", ALPINE, "cat", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("foobar"))
	})

	It("podman run mutually excludes --dns* and --network", func() {
		session := podmanTest.Podman([]string{"run", "--dns=1.2.3.4", "--network", "container:ALPINE", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"run", "--dns-opt=1.2.3.4", "--network", "container:ALPINE", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"run", "--dns-search=foobar.com", "--network", "none", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"run", "--dns=1.2.3.4", "--network", "host", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
})
