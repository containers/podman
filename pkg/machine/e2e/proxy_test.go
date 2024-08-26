package e2e_test

import (
	"os"
	"path/filepath"

	"github.com/containers/podman/v5/pkg/machine/define"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine proxy settings propagation", func() {

	It("ssh to running machine and check proxy settings", func() {
		defer func() {
			os.Unsetenv("HTTP_PROXY")
			os.Unsetenv("HTTPS_PROXY")
			os.Unsetenv("SSL_CERT_DIR")
			os.Unsetenv("SSL_CERT_FILE")
		}()

		certFileDir := GinkgoT().TempDir()
		certDir := GinkgoT().TempDir()
		certFile := filepath.Join(certFileDir, "cert1")
		err := os.WriteFile(certFile, []byte("cert1 content\n"), os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(filepath.Join(certDir, "cert2"), []byte("cert2 content\n"), os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("SSL_CERT_FILE", certFile)
		os.Setenv("SSL_CERT_DIR", certDir)

		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

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

		// SSL_CERT not implemented for WSL
		if !isVmtype(define.WSLVirt) {
			sshSession, err = mb.setName(name).setCmd(sshProxy.withSSHCommand([]string{"printenv", "SSL_CERT_DIR", "SSL_CERT_FILE"})).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(sshSession).To(Exit(0))
			Expect(string(sshSession.Out.Contents())).To(Equal(define.UserCertsTargetPath + "\n" + define.UserCertsTargetPath + "/cert1" + "\n"))

			sshSession, err = mb.setName(name).setCmd(sshProxy.withSSHCommand([]string{"cat", "$SSL_CERT_DIR/cert2", "$SSL_CERT_FILE"})).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(sshSession).To(Exit(0))
			Expect(string(sshSession.Out.Contents())).To(Equal("cert2 content\ncert1 content\n"))
		}

		stop := new(stopMachine)
		stopSession, err := mb.setName(name).setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		// Now update proxy env, lets use some special vars to make sure our scripts can handle it
		proxy1 := "http://foo:b%%40r@example.com:8080"
		proxy2 := "https://foo:bar%%3F@example.com:8080"
		noproxy := "noproxy1.example.com,noproxy2.example.com"
		os.Setenv("HTTP_PROXY", proxy1)
		os.Setenv("HTTPS_PROXY", proxy2)
		os.Setenv("NO_PROXY", noproxy)

		// changing SSL_CERT vars should not have an effect
		os.Setenv("SSL_CERT_FILE", "/tmp/1")
		os.Setenv("SSL_CERT_DIR", "/tmp")

		// start it again should update the proxies
		startSession, err = mb.setName(name).setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		sshSession, err = mb.setName(name).setCmd(sshProxy.withSSHCommand([]string{"printenv", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		Expect(string(sshSession.Out.Contents())).To(Equal(proxy1 + "\n" + proxy2 + "\n" + noproxy + "\n"))

		// SSL_CERT not implemented for WSL
		if !isVmtype(define.WSLVirt) {
			// SSL_CERT... must still be the same as before
			sshSession, err = mb.setName(name).setCmd(sshProxy.withSSHCommand([]string{"cat", "$SSL_CERT_DIR/cert2", "$SSL_CERT_FILE"})).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(sshSession).To(Exit(0))
			Expect(string(sshSession.Out.Contents())).To(Equal("cert2 content\ncert1 content\n"))
		}
	})
})
