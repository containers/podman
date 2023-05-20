package integration

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

// podman unshare --rootless-netns leaks the process by design.
// Running a container will cause the cleanup to kick in when this container gets stopped.
func cleanupRootlessSlirp4netns(p *PodmanTestIntegration) {
	session := p.Podman([]string{"run", "--network", "bridge", ALPINE, "true"})
	session.WaitWithDefaultTimeout()
	Expect(session).Should(Exit(0))
}

var _ = Describe("Podman unshare", func() {
	BeforeEach(func() {
		if _, err := os.Stat("/proc/self/uid_map"); err != nil {
			Skip("User namespaces not supported.")
		}

		if !isRootless() {
			Skip("Use unshare in rootless only")
		}
	})

	It("podman unshare", func() {
		SkipIfRemote("podman-remote unshare is not supported")
		userNS, _ := os.Readlink("/proc/self/ns/user")
		session := podmanTest.Podman([]string{"unshare", "readlink", "/proc/self/ns/user"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).ToNot(ContainSubstring(userNS))
	})

	It("podman unshare --rootless-netns", func() {
		SkipIfRemote("podman-remote unshare is not supported")
		defer cleanupRootlessSlirp4netns(podmanTest)
		session := podmanTest.Podman([]string{"unshare", "--rootless-netns", "ip", "addr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("tap0"))
	})

	It("podman unshare exit codes", func() {
		SkipIfRemote("podman-remote unshare is not supported")
		session := podmanTest.Podman([]string{"unshare", "false"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
		Expect(session.OutputToString()).Should(Equal(""))
		Expect(session.ErrorToString()).Should(Equal(""))

		session = podmanTest.Podman([]string{"unshare", "/usr/bin/bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(127))
		Expect(session.OutputToString()).Should(Equal(""))
		Expect(session.ErrorToString()).Should(ContainSubstring("no such file or directory"))

		session = podmanTest.Podman([]string{"unshare", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(127))
		Expect(session.OutputToString()).Should(Equal(""))
		Expect(session.ErrorToString()).Should(ContainSubstring("executable file not found in $PATH"))

		session = podmanTest.Podman([]string{"unshare", "/usr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(126))
		Expect(session.OutputToString()).Should(Equal(""))
		Expect(session.ErrorToString()).Should(ContainSubstring("permission denied"))

		session = podmanTest.Podman([]string{"unshare", "--bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.OutputToString()).Should(Equal(""))
		Expect(session.ErrorToString()).Should(ContainSubstring("unknown flag: --bogus"))
	})

	It("podman unshare check remote error", func() {
		SkipIfNotRemote("check for podman-remote unshare error")
		session := podmanTest.Podman([]string{"unshare"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(Equal(`Error: cannot use command "podman-remote unshare" with the remote podman client`))
	})
})
