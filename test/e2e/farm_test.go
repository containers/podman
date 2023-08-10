package integration

import (
	"os"
	"path/filepath"

	"github.com/containers/common/pkg/config"
	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

func setupContainersConfWithSystemConnections() {
	// make sure connections are not written to real user config on host
	file := filepath.Join(podmanTest.TempDir, "containersconf")
	f, err := os.Create(file)
	Expect(err).ToNot(HaveOccurred())
	connections := `
[engine]
active_service = "QA"
[engine.service_destinations]
	[engine.service_destinations.QA]
	uri = "ssh://root@podman.test:2222/run/podman/podman.sock"
	[engine.service_destinations.QB]
	uri = "ssh://root@podman.test:3333/run/podman/podman.sock"`
	_, err = f.WriteString(connections)
	Expect(err).ToNot(HaveOccurred())
	f.Close()
	os.Setenv("CONTAINERS_CONF", file)
}

var _ = Describe("podman farm", func() {

	BeforeEach(setupContainersConfWithSystemConnections)

	Context("without running API service", func() {
		It("verify system connections exist", func() {
			cfg, err := config.ReadCustomConfig()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg).Should(HaveActiveService("QA"))
			Expect(cfg).Should(VerifyService(
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
				"",
			))
			Expect(cfg).Should(VerifyService(
				"QB",
				"ssh://root@podman.test:3333/run/podman/podman.sock",
				"",
			))
		})

		It("create farm", func() {
			// create farm with multiple system connections
			cmd := []string{"farm", "create", "farm1", "QA", "QB"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" created"))

			// create farm with only one system connection
			cmd = []string{"farm", "create", "farm2", "QA"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" created"))

			// create empty farm
			cmd = []string{"farm", "create", "farm3"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm3\" created"))

			cfg, err := config.ReadCustomConfig()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg.Farms.Default).Should(Equal("farm1"))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm1", []string{"QA", "QB"}))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm2", []string{"QA"}))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm3", []string{}))

			// create existing farm should exit with error
			cmd = []string{"farm", "create", "farm3"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Not(Exit(0)))
		})

		It("update existing farms", func() {
			// create farm with multiple system connections
			cmd := []string{"farm", "create", "farm1", "QA", "QB"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" created"))

			// create farm with only one system connection
			cmd = []string{"farm", "create", "farm2", "QA"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" created"))

			// create empty farm
			cmd = []string{"farm", "create", "farm3"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm3\" created"))

			cfg, err := config.ReadCustomConfig()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg.Farms.Default).Should(Equal("farm1"))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm1", []string{"QA", "QB"}))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm2", []string{"QA"}))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm3", []string{}))

			// update farm1 to remove the QA connection from it
			cmd = []string{"farm", "update", "--remove", "QA,QB", "farm1"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" updated"))

			// update farm3 to add QA and QB connections to it
			cmd = []string{"farm", "update", "--add", "QB", "farm3"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm3\" updated"))

			// update farm2 to be the default farm
			cmd = []string{"farm", "update", "--default", "farm2"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" updated"))

			cfg, err = config.ReadCustomConfig()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg.Farms.Default).Should(Equal("farm2"))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm1", []string{}))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm2", []string{"QA"}))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm3", []string{"QB"}))

			// update farm2 to not be the default, no farms should be the default
			cmd = []string{"farm", "update", "--default=false", "farm2"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" updated"))

			cfg, err = config.ReadCustomConfig()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg.Farms.Default).Should(BeEmpty())
		})

		It("remove existing farms", func() {
			// create farm with multiple system connections
			cmd := []string{"farm", "create", "farm1", "QA", "QB"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" created"))

			// create farm with only one system connection
			cmd = []string{"farm", "create", "farm2", "QA"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" created"))

			cfg, err := config.ReadCustomConfig()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg.Farms.Default).Should(Equal("farm1"))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm1", []string{"QA", "QB"}))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm2", []string{"QA"}))

			// remove farm1 and a non-existent farm
			// farm 1 should be removed and a warning printed for nonexistent-farm
			cmd = []string{"farm", "rm", "farm1", "nonexistent-farm"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" deleted"))

			cfg, err = config.ReadCustomConfig()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg.Farms.Default).Should(Equal("farm2"))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm2", []string{"QA"}))
			Expect(cfg.Farms.List).Should(Not(HaveKey("farm1")))

			// remove all non-existent farms and expect an error
			cmd = []string{"farm", "rm", "foo", "bar"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Not(Exit(0)))
		})

		It("remove --all farms", func() {
			// create farm with multiple system connections
			cmd := []string{"farm", "create", "farm1", "QA", "QB"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm1\" created"))

			// create farm with only one system connection
			cmd = []string{"farm", "create", "farm2", "QA"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"farm2\" created"))

			cfg, err := config.ReadCustomConfig()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg.Farms.Default).Should(Equal("farm1"))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm1", []string{"QA", "QB"}))
			Expect(cfg.Farms.List).Should(HaveKeyWithValue("farm2", []string{"QA"}))

			// remove --all
			cmd = []string{"farm", "rm", "--all"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(ContainSubstring("All farms have been deleted"))

			cfg, err = config.ReadCustomConfig()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg.Farms.Default).Should(BeEmpty())
			Expect(cfg.Farms.List).Should(BeEmpty())
		})
	})
})
