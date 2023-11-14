package e2e_test

import (
	"os"

	"github.com/containers/podman/v4/pkg/machine/define"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine proxy settings propagation", func() {
	var (
		mb      *machineTestBuilder
		testDir string
	)

	BeforeEach(func() {
		testDir, mb = setup()
	})
	AfterEach(func() {
		teardown(originalHomeDir, testDir, mb)
	})

	It("ssh to running machine and check proxy settings", func() {
		// https://github.com/containers/podman/issues/20129
		if testProvider.VMType() == define.HyperVVirt {
			Skip("proxy settings not yet supported")
		}
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		defer func() {
			httpProxyEnv := os.Getenv("HTTP_PROXY")
			httpsProxyEnv := os.Getenv("HTTPS_PROXY")
			if httpProxyEnv != "" {
				os.Unsetenv("HTTP_PROXY")
			}
			if httpsProxyEnv != "" {
				os.Unsetenv("HTTPS_PROXY")
			}
		}()
		proxyURL := "http://abcdefghijklmnopqrstuvwxyz-proxy"
		os.Setenv("HTTP_PROXY", proxyURL)
		os.Setenv("HTTPS_PROXY", proxyURL)

		s := new(startMachine)
		startSession, err := mb.setName(name).setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		sshProxy := sshMachine{}
		sshSession, err := mb.setName(name).setCmd(sshProxy.withSSHCommand([]string{"printenv", "HTTP_PROXY"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		Expect(sshSession.outputToString()).To(ContainSubstring(proxyURL))

		sshSession, err = mb.setName(name).setCmd(sshProxy.withSSHCommand([]string{"printenv", "HTTPS_PROXY"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		Expect(sshSession.outputToString()).To(ContainSubstring(proxyURL))
	})
})
