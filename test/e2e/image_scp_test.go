package integration

import (
	"os"
	"path/filepath"

	"github.com/containers/common/pkg/config"
	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/storage/pkg/homedir"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman image scp", func() {
	ConfPath := struct {
		Value string
		IsSet bool
	}{}
	var (
		tempdir    string
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		ConfPath.Value, ConfPath.IsSet = os.LookupEnv("CONTAINERS_CONF")
		conf, err := os.CreateTemp("", "containersconf")
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("CONTAINERS_CONF", conf.Name())
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()

		os.Remove(os.Getenv("CONTAINERS_CONF"))
		if ConfPath.IsSet {
			os.Setenv("CONTAINERS_CONF", ConfPath.Value)
		} else {
			os.Unsetenv("CONTAINERS_CONF")
		}
		f := CurrentSpecReport()
		processTestResult(f)

	})

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
		Expect(session).Should(Exit(0))

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
