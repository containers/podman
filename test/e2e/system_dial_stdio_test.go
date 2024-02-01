package integration

import (
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman system dial-stdio", func() {

	It("podman system dial-stdio help", func() {
		session := podmanTest.Podman([]string{"system", "dial-stdio", "--help"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("Examples: podman system dial-stdio"))
	})

	It("podman system dial-stdio while service is not running", func() {
		if IsRemote() {
			Skip("this test is only for non-remote")
		}
		session := podmanTest.Podman([]string{"system", "dial-stdio"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("Error: failed to open connection to podman"))
	})
})
