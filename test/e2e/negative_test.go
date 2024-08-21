//go:build linux || freebsd

package integration

import (
	"fmt"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman negative command-line", func() {

	It("podman snuffleupagus exits non-zero", func() {
		session := podmanTest.Podman([]string{"snuffleupagus"})
		session.WaitWithDefaultTimeout()
		cmdName := "podman"
		if IsRemote() {
			cmdName += "-remote"
		}
		Expect(session).To(ExitWithError(125, fmt.Sprintf("unrecognized command `%s snuffleupagus`", cmdName)))
	})
})
