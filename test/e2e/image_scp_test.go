package integration

import (
	"os"
	"path/filepath"

	"github.com/containers/common/pkg/config"
	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/storage/pkg/homedir"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("podman image scp", func() {

	BeforeEach(setupEmptyContainersConf)

	It("podman image scp bogus image", func() {
		scp := podmanTest.Podman([]string{"image", "scp", "FOOBAR"})
		scp.WaitWithDefaultTimeout()
		Expect(scp).Should(ExitWithError())
	})

	It("podman image scp with proper connection", func() {
		if _, err := os.Stat(filepath.Join(homedir.Get(), ".ssh", "known_hosts")); err != nil {
			Skip("known_hosts does not exist or is not accessible")
		}
		cmd := []string{"system", "connection", "add",
			"--default",
			"QA",
			"ssh://root@podman.test:2222/run/podman/podman.sock",
		}
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		cfg, err := config.ReadCustomConfig()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cfg.Engine).Should(HaveField("ActiveService", "QA"))
		Expect(cfg.Engine.ServiceDestinations).To(HaveKeyWithValue("QA",
			config.Destination{
				URI: "ssh://root@podman.test:2222/run/podman/podman.sock",
			},
		))

		scp := podmanTest.Podman([]string{"image", "scp", ALPINE, "QA::"})
		scp.WaitWithDefaultTimeout()
		// exit with error because we cannot make an actual ssh connection
		// This tests that the input we are given is validated and prepared correctly
		// The error given should either be a missing image (due to testing suite complications) or a no such host timeout on ssh
		Expect(scp).Should(ExitWithError())
		// podman-remote exits with a different error
		if !IsRemote() {
			Expect(scp.ErrorToString()).Should(ContainSubstring("no such host"))
		}

	})

})
