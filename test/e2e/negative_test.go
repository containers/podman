package integration

import (
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman negative command-line", func() {

	It("podman snuffleupagus exits non-zero", func() {
		session := podmanTest.Podman([]string{"snuffleupagus"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})
})
