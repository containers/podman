package integration

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/containers/common/pkg/config"
	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman system connection", func() {
	ConfPath := struct {
		Value string
		IsSet bool
	}{}

	var podmanTest *PodmanTestIntegration

	BeforeEach(func() {
		ConfPath.Value, ConfPath.IsSet = os.LookupEnv("CONTAINERS_CONF")
		conf, err := os.CreateTemp("", "containersconf")
		Expect(err).ToNot(HaveOccurred())
		os.Setenv("CONTAINERS_CONF", conf.Name())

		tempdir, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())
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

	Context("without running API service", func() {
		It("add ssh://", func() {
			cmd := []string{"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(BeEmpty())

			cfg, err := config.ReadCustomConfig()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg).Should(HaveActiveService("QA"))
			Expect(cfg).Should(VerifyService(
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
				"~/.ssh/id_rsa",
			))

			cmd = []string{"system", "connection", "rename",
				"QA",
				"QE",
			}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			Expect(config.ReadCustomConfig()).Should(HaveActiveService("QE"))
		})

		It("add UDS", func() {
			cmd := []string{"system", "connection", "add",
				"QA-UDS",
				"unix:///run/podman/podman.sock",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(BeEmpty())

			Expect(config.ReadCustomConfig()).Should(VerifyService(
				"QA-UDS",
				"unix:///run/podman/podman.sock",
				"",
			))

			cmd = []string{"system", "connection", "add",
				"QA-UDS1",
				"--socket-path", "/run/user/podman/podman.sock",
				"unix:///run/podman/podman.sock",
			}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(BeEmpty())

			Expect(config.ReadCustomConfig()).Should(HaveActiveService("QA-UDS"))
			Expect(config.ReadCustomConfig()).Should(VerifyService(
				"QA-UDS1",
				"unix:///run/user/podman/podman.sock",
				"",
			))
		})

		It("add tcp", func() {
			cmd := []string{"system", "connection", "add",
				"QA-TCP",
				"tcp://localhost:8888",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(BeEmpty())

			Expect(config.ReadCustomConfig()).Should(VerifyService(
				"QA-TCP",
				"tcp://localhost:8888",
				"",
			))
		})

		It("remove", func() {
			session := podmanTest.Podman([]string{"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			// two passes to test that removing non-existent connection is not an error
			for i := 0; i < 2; i++ {
				session = podmanTest.Podman([]string{"system", "connection", "remove", "QA"})
				session.WaitWithDefaultTimeout()
				Expect(session).Should(Exit(0))
				Expect(session.Out.Contents()).Should(BeEmpty())

				cfg, err := config.ReadCustomConfig()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(cfg.Engine.ActiveService).Should(BeEmpty())
				Expect(cfg.Engine.ServiceDestinations).Should(BeEmpty())
			}
		})

		It("remove --all", func() {
			session := podmanTest.Podman([]string{"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			session = podmanTest.Podman([]string{"system", "connection", "remove", "--all"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(BeEmpty())
			Expect(session.Err.Contents()).Should(BeEmpty())

			session = podmanTest.Podman([]string{"system", "connection", "list"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		})

		It("default", func() {
			for _, name := range []string{"devl", "qe"} {
				cmd := []string{"system", "connection", "add",
					"--default",
					"--identity", "~/.ssh/id_rsa",
					name,
					"ssh://root@podman.test:2222/run/podman/podman.sock",
				}
				session := podmanTest.Podman(cmd)
				session.WaitWithDefaultTimeout()
				Expect(session).Should(Exit(0))
			}

			cmd := []string{"system", "connection", "default", "devl"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(BeEmpty())

			Expect(config.ReadCustomConfig()).Should(HaveActiveService("devl"))

			cmd = []string{"system", "connection", "list"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out).Should(Say("Name *URI *Identity *Default"))

			cmd = []string{"system", "connection", "list", "--format", "{{.Name}}"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.OutputToString()).Should(Equal("devl qe"))
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
			Expect(session.OutputToStringArray()).Should(HaveLen(1))
			Expect(session.Err.Contents()).Should(BeEmpty())
		})
	})

	Context("sshd and API services required", func() {
		BeforeEach(func() {
			// These tests are unique in as much as they require podman, podman-remote, systemd and sshd.
			// podman-remote commands will be executed by ginkgo directly.
			SkipIfContainerized("sshd is not available when running in a container")
			SkipIfRemote("connection heuristic requires both podman and podman-remote binaries")
			SkipIfNotRootless(fmt.Sprintf("FIXME: set up ssh keys when root. uid(%d) euid(%d)", os.Getuid(), os.Geteuid()))
			SkipIfSystemdNotRunning("cannot test connection heuristic if systemd is not running")
			SkipIfNotActive("sshd", "cannot test connection heuristic if sshd is not running")
		})

		It("add ssh:// socket path using connection heuristic", func() {
			u, err := user.Current()
			Expect(err).ShouldNot(HaveOccurred())

			// Ensure that the remote end uses our built podman
			if os.Getenv("PODMAN_BINARY") == "" {
				err = os.Setenv("PODMAN_BINARY", podmanTest.PodmanBinary)
				Expect(err).ShouldNot(HaveOccurred())

				defer func() {
					os.Unsetenv("PODMAN_BINARY")
				}()
			}

			cmd := exec.Command(podmanTest.RemotePodmanBinary,
				"system", "connection", "add",
				"--default",
				"--identity", filepath.Join(u.HomeDir, ".ssh", "id_ed25519"),
				"QA",
				fmt.Sprintf("ssh://%s@localhost", u.Username))

			session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%q failed to execute", podmanTest.RemotePodmanBinary))
			Eventually(session, DefaultWaitTimeout).Should(Exit(0))
			Expect(session.Out.Contents()).Should(BeEmpty())
			Expect(session.Err.Contents()).Should(BeEmpty())

			cmd = exec.Command(podmanTest.RemotePodmanBinary,
				"--connection", "QA", "ps")
			_, err = Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			// export the container_host env var and try again
			err = os.Setenv("CONTAINER_HOST", fmt.Sprintf("ssh://%s@localhost", u.Username))
			Expect(err).ToNot(HaveOccurred())
			defer os.Unsetenv("CONTAINER_HOST")

			cmd = exec.Command(podmanTest.RemotePodmanBinary, "ps")
			_, err = Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			uri := url.URL{
				Scheme: "ssh",
				User:   url.User(u.Username),
				Host:   "localhost:22",
				Path:   fmt.Sprintf("/run/user/%s/podman/podman.sock", u.Uid),
			}

			Expect(config.ReadCustomConfig()).Should(HaveActiveService("QA"))
			Expect(config.ReadCustomConfig()).Should(VerifyService(
				"QA",
				uri.String(),
				filepath.Join(u.HomeDir, ".ssh", "id_ed25519"),
			))
		})
	})
})
