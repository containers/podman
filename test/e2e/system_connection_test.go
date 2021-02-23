package integration

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/containers/common/pkg/config"
	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman system connection", func() {
	ConfPath := struct {
		Value string
		IsSet bool
	}{}

	var (
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		ConfPath.Value, ConfPath.IsSet = os.LookupEnv("CONTAINERS_CONF")
		conf, err := ioutil.TempFile("", "containersconf")
		if err != nil {
			panic(err)
		}
		os.Setenv("CONTAINERS_CONF", conf.Name())

		tempdir, err := CreateTempDirInTempDir()
		if err != nil {
			panic(err)
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

		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("add", func() {
		cmd := []string{"system", "connection", "add",
			"--default",
			"--identity", "~/.ssh/id_rsa",
			"QA",
			"ssh://root@server.fubar.com:2222/run/podman/podman.sock",
		}
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.Out).Should(Say(""))

		cfg, err := config.ReadCustomConfig()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cfg.Engine.ActiveService).To(Equal("QA"))
		Expect(cfg.Engine.ServiceDestinations["QA"]).To(Equal(
			config.Destination{
				URI:      "ssh://root@server.fubar.com:2222/run/podman/podman.sock",
				Identity: "~/.ssh/id_rsa",
			},
		))

		cmd = []string{"system", "connection", "rename",
			"QA",
			"QE",
		}
		session = podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		cfg, err = config.ReadCustomConfig()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cfg.Engine.ActiveService).To(Equal("QE"))
		Expect(cfg.Engine.ServiceDestinations["QE"]).To(Equal(
			config.Destination{
				URI:      "ssh://root@server.fubar.com:2222/run/podman/podman.sock",
				Identity: "~/.ssh/id_rsa",
			},
		))
	})

	It("remove", func() {
		cmd := []string{"system", "connection", "add",
			"--default",
			"--identity", "~/.ssh/id_rsa",
			"QA",
			"ssh://root@server.fubar.com:2222/run/podman/podman.sock",
		}
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		for i := 0; i < 2; i++ {
			cmd = []string{"system", "connection", "remove", "QA"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out).Should(Say(""))

			cfg, err := config.ReadCustomConfig()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg.Engine.ActiveService).To(BeEmpty())
			Expect(cfg.Engine.ServiceDestinations).To(BeEmpty())
		}
	})

	It("default", func() {
		for _, name := range []string{"devl", "qe"} {
			cmd := []string{"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				name,
				"ssh://root@server.fubar.com:2222/run/podman/podman.sock",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}

		cmd := []string{"system", "connection", "default", "devl"}
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.Out).Should(Say(""))

		cfg, err := config.ReadCustomConfig()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cfg.Engine.ActiveService).To(Equal("devl"))

		cmd = []string{"system", "connection", "list"}
		session = podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.Out).Should(Say("Name *Identity *URI"))
	})

	It("failed default", func() {
		cmd := []string{"system", "connection", "default", "devl"}
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).ShouldNot(Exit(0))
		Expect(session.Err).Should(Say("destination is not defined"))
	})

	It("failed rename", func() {
		cmd := []string{"system", "connection", "rename", "devl", "QE"}
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).ShouldNot(Exit(0))
		Expect(session.Err).Should(Say("destination is not defined"))
	})

	It("empty list", func() {
		cmd := []string{"system", "connection", "list"}
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.Out).Should(Say(""))
		Expect(session.Err).Should(Say(""))
	})
})
