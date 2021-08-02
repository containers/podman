package integration

import (
	"io/ioutil"
	"os"

	"github.com/containers/common/pkg/config"
	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
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
		conf, err := ioutil.TempFile("", "containersconf")
		if err != nil {
			panic(err)
		}
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
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman image scp quiet flag", func() {
		if IsRemote() {
			Skip("this test is only for non-remote")
		}
		scp := podmanTest.Podman([]string{"image", "scp", "-q", ALPINE})
		scp.WaitWithDefaultTimeout()
		Expect(scp).To(Exit(0))
	})

	It("podman image scp bogus image", func() {
		if IsRemote() {
			Skip("this test is only for non-remote")
		}
		scp := podmanTest.Podman([]string{"image", "scp", "FOOBAR"})
		scp.WaitWithDefaultTimeout()
		Expect(scp).To(ExitWithError())
	})

	It("podman image scp with proper connection", func() {
		if IsRemote() {
			Skip("this test is only for non-remote")
		}
		cmd := []string{"system", "connection", "add",
			"--default",
			"QA",
			"ssh://root@server.fubar.com:2222/run/podman/podman.sock",
		}
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).To(Exit(0))

		cfg, err := config.ReadCustomConfig()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cfg.Engine.ActiveService).To(Equal("QA"))
		Expect(cfg.Engine.ServiceDestinations["QA"]).To(Equal(
			config.Destination{
				URI: "ssh://root@server.fubar.com:2222/run/podman/podman.sock",
			},
		))

		scp := podmanTest.Podman([]string{"image", "scp", ALPINE, "QA::"})
		scp.Wait(45)
		// exit with error because we cannot make an actual ssh connection
		// This tests that the input we are given is validated and prepared correctly
		// Error: failed to connect: dial tcp: address foo: missing port in address
		Expect(scp).To(ExitWithError())
		Expect(scp.ErrorToString()).To(ContainSubstring(
			"Error: failed to connect: dial tcp 66.151.147.142:2222: i/o timeout",
		))

	})

})
