//go:build linux || freebsd

package integration

import (
	. "github.com/containers/podman/v6/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run dns", func() {
	It("podman run add search domain", func() {
		session := podmanTest.PodmanExitCleanly("run", "--dns-search=foobar.com", ALPINE, "cat", "/etc/resolv.conf")
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("search foobar.com")))
	})

	It("podman run remove all search domain", func() {
		session := podmanTest.PodmanExitCleanly("run", "--dns-search=.", ALPINE, "cat", "/etc/resolv.conf")
		Expect(session.OutputToStringArray()).To(Not(ContainElement(HavePrefix("search"))))
	})

	It("podman run add bad dns server", func() {
		session := podmanTest.Podman([]string{"run", "--dns=foobar", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "foobar is not an ip address"))
	})

	It("podman run add dns server", func() {
		session := podmanTest.PodmanExitCleanly("run", "--dns=1.2.3.4", ALPINE, "cat", "/etc/resolv.conf")
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("nameserver 1.2.3.4")))
	})

	It("podman run add dns option", func() {
		session := podmanTest.PodmanExitCleanly("run", "--dns-opt=debug", ALPINE, "cat", "/etc/resolv.conf")
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("options debug")))
	})

	It("podman run add bad host", func() {
		session := podmanTest.Podman([]string{"run", "--add-host=foo:1.2", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `invalid IP address in add-host: "1.2"`))
	})

	It("podman run add host", func() {
		session := podmanTest.PodmanExitCleanly("run", "--add-host=foobar:1.1.1.1", "--add-host=foobaz:2001:db8::68", ALPINE, "cat", "/etc/hosts")
		Expect(session.OutputToStringArray()).To(ContainElements(HavePrefix("1.1.1.1\tfoobar"), HavePrefix("2001:db8::68\tfoobaz")))
	})

	It("podman run add hostname", func() {
		session := podmanTest.PodmanExitCleanly("run", "--hostname=foobar", ALPINE, "cat", "/etc/hostname")
		Expect(string(session.Out.Contents())).To(Equal("foobar\n"))

		session = podmanTest.PodmanExitCleanly("run", "--hostname=foobar", ALPINE, "hostname")
		Expect(session.OutputToString()).To(Equal("foobar"))
	})

	It("podman run add hostname sets /etc/hosts", func() {
		session := podmanTest.PodmanExitCleanly("run", "--hostname=foobar", ALPINE, "cat", "/etc/hosts")
		Expect(session.OutputToString()).To(ContainSubstring("foobar"))
	})

	It("podman run --dns with --network none is rejected", func() {
		session := podmanTest.Podman([]string{"run", "--dns=1.2.3.4", "--network", "none", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "conflicting options: dns and the network mode: none"))
	})

	It("podman run --dns-search with --network none is rejected", func() {
		session := podmanTest.Podman([]string{"run", "--dns-search=foobar.com", "--network", "none", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "conflicting options: dns and the network mode: none"))
	})

	It("podman run --dns-opt with --network none is rejected", func() {
		session := podmanTest.Podman([]string{"run", "--dns-opt=debug", "--network", "none", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "conflicting options: dns and the network mode: none"))
	})

	It("podman run --dns with --network host is allowed", func() {
		podmanTest.PodmanExitCleanly("run", "--dns=1.2.3.4", "--network", "host", ALPINE)
	})

	It("podman run --dns with --network container works", func() {
		ctrName := "dnsContainerSrc"
		podmanTest.PodmanExitCleanly("run", "-d", "--name", ctrName, "--dns=1.1.1.1", ALPINE, "top")

		session := podmanTest.PodmanExitCleanly("run", "--dns=8.8.8.8", "--network", "container:"+ctrName, ALPINE, "cat", "/etc/resolv.conf")
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("nameserver 8.8.8.8")))
		Expect(session.OutputToStringArray()).To(Not(ContainElement(HavePrefix("nameserver 1.1.1.1"))))
	})

	It("podman run --network container without dns flags keeps dependency resolv.conf", func() {
		ctrName := "dnsContainerDefaultShareSrc"
		podmanTest.PodmanExitCleanly("run", "-d", "--name", ctrName, "--dns=1.1.1.1", ALPINE, "top")

		session := podmanTest.PodmanExitCleanly("run", "--network", "container:"+ctrName, ALPINE, "cat", "/etc/resolv.conf")
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("nameserver 1.1.1.1")))
	})

	It("podman run --dns-search with --network container inherits dependency nameserver", func() {
		ctrName := "dnsSearchContainerSrc"
		podmanTest.PodmanExitCleanly("run", "-d", "--name", ctrName, "--dns=1.1.1.1", ALPINE, "top")

		session := podmanTest.PodmanExitCleanly("run", "--dns-search=example.com", "--network", "container:"+ctrName, ALPINE, "cat", "/etc/resolv.conf")
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("nameserver 1.1.1.1")))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("search example.com")))
	})

	It("podman run --dns-search with --network container works when dependency has --dns none", func() {
		ctrName := "dnsNoneContainerSrc"
		podmanTest.PodmanExitCleanly("run", "-d", "--name", ctrName, "--dns", "none", ALPINE, "top")

		session := podmanTest.PodmanExitCleanly("run", "--dns-search=example.com", "--network", "container:"+ctrName, ALPINE, "cat", "/etc/resolv.conf")
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("nameserver ")))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("search example.com")))
	})

	It("podman run --dns-opt with --network container works when dependency has --dns none", func() {
		ctrName := "dnsOptNoneContainerSrc"
		podmanTest.PodmanExitCleanly("run", "-d", "--name", ctrName, "--dns", "none", ALPINE, "top")

		session := podmanTest.PodmanExitCleanly("run", "--dns-opt=debug", "--network", "container:"+ctrName, ALPINE, "cat", "/etc/resolv.conf")
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("nameserver ")))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("options debug")))
	})

	It("podman run --dns-search and --dns-opt with --network container work when dependency has --dns none", func() {
		ctrName := "dnsCombinedNoneContainerSrc"
		podmanTest.PodmanExitCleanly("run", "-d", "--name", ctrName, "--dns", "none", ALPINE, "top")

		session := podmanTest.PodmanExitCleanly(
			"run",
			"--dns-search=example.com",
			"--dns-opt=debug",
			"--network",
			"container:"+ctrName,
			ALPINE,
			"cat",
			"/etc/resolv.conf",
		)
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("nameserver ")))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("search example.com")))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("options debug")))
	})

	It("podman run --dns-opt with --network container inherits dependency nameserver", func() {
		ctrName := "dnsOptContainerSrc"
		podmanTest.PodmanExitCleanly("run", "-d", "--name", ctrName, "--dns=1.1.1.1", ALPINE, "top")

		session := podmanTest.PodmanExitCleanly("run", "--dns-opt=debug", "--network", "container:"+ctrName, ALPINE, "cat", "/etc/resolv.conf")
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("nameserver 1.1.1.1")))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("options debug")))
	})

	It("podman run combined dns flags with --network container works", func() {
		ctrName := "dnsCombinedContainerSrc"
		podmanTest.PodmanExitCleanly("run", "-d", "--name", ctrName, ALPINE, "top")

		session := podmanTest.PodmanExitCleanly(
			"run",
			"--dns=8.8.8.8",
			"--dns-search=example.com",
			"--dns-opt=debug",
			"--network",
			"container:"+ctrName,
			ALPINE,
			"cat",
			"/etc/resolv.conf",
		)
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("nameserver 8.8.8.8")))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("search example.com")))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("options debug")))
	})
})
