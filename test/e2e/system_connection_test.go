package integration

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

func setupConnectionsConf() {
	// make sure connections are not written to real user config on host
	file := filepath.Join(podmanTest.TempDir, "containers.conf")
	f, err := os.Create(file)
	Expect(err).ToNot(HaveOccurred())
	f.Close()
	os.Setenv("CONTAINERS_CONF", file)

	file = filepath.Join(podmanTest.TempDir, "connections.conf")
	os.Setenv("PODMAN_CONNECTIONS_CONF", file)
}

var systemConnectionListCmd = []string{"system", "connection", "ls", "--format", "{{.Name}} {{.URI}} {{.Identity}} {{.Default}} {{.ReadWrite}}"}
var farmListCmd = []string{"farm", "ls", "--format", "{{.Name}} {{.Connections}} {{.Default}} {{.ReadWrite}}"}

var _ = Describe("podman system connection", func() {

	BeforeEach(setupConnectionsConf)

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
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa true true"))

			cmd = []string{"system", "connection", "rename",
				"QA",
				"QE",
			}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QE ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa true true"))
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
			// stderr will probably warn (ENOENT or EACCESS) about socket
			// but it's too unreliable to test for.

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA-UDS unix:///run/podman/podman.sock true true"))

			cmd = []string{"system", "connection", "add",
				"QA-UDS1",
				"--socket-path", "/run/user/podman/podman.sock",
				"unix:///run/podman/podman.sock",
			}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(string(session.Out.Contents())).To(Equal(`QA-UDS unix:///run/podman/podman.sock  true true
QA-UDS1 unix:///run/user/podman/podman.sock  false true
`))
		})

		It("add tcp", func() {
			cmd := []string{"system", "connection", "add",
				"QA-TCP",
				"tcp://localhost:8888",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA-TCP tcp://localhost:8888 true true"))
		})

		It("add to new farm", func() {
			cmd := []string{"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"--farm", "farm1",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa true true"))
			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("farm1 [QA] true true"))
		})

		It("add to existing farm", func() {
			// create empty farm
			cmd := []string{"farm", "create", "empty-farm"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(ContainSubstring("Farm \"empty-farm\" created"))

			cmd = []string{"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"--farm", "empty-farm",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa true true"))
			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("empty-farm [QA] true true"))
		})

		It("removing connection should remove from farm also", func() {
			cmd := []string{"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"--farm", "farm1",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("QA ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa true true"))
			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("farm1 [QA] true true"))

			// Remove the QA connection
			session = podmanTest.Podman([]string{"system", "connection", "remove", "QA"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal(""))
			session = podmanTest.Podman(farmListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("farm1 [] true true"))
		})

		It("remove", func() {
			session := podmanTest.Podman([]string{"system", "connection", "add",
				"--default",
				"--identity", "~/.ssh/id_rsa",
				"QA",
				"ssh://root@podman.test:2222/run/podman/podman.sock",
			})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			// two passes to test that removing non-existent connection is not an error
			for i := 0; i < 2; i++ {
				session = podmanTest.Podman([]string{"system", "connection", "remove", "QA"})
				session.WaitWithDefaultTimeout()
				Expect(session).Should(ExitCleanly())
				Expect(session.Out.Contents()).Should(BeEmpty())

				session = podmanTest.Podman(systemConnectionListCmd)
				session.WaitWithDefaultTimeout()
				Expect(session).Should(ExitCleanly())
				Expect(session.OutputToString()).To(Equal(""))
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
			Expect(session).Should(ExitCleanly())

			session = podmanTest.Podman([]string{"system", "connection", "remove", "--all"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())
			Expect(session.Err.Contents()).Should(BeEmpty())

			session = podmanTest.Podman([]string{"system", "connection", "list"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToStringArray()).To(HaveLen(1))
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
				Expect(session).Should(ExitCleanly())
			}

			cmd := []string{"system", "connection", "default", "devl"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out.Contents()).Should(BeEmpty())

			session = podmanTest.Podman(systemConnectionListCmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(string(session.Out.Contents())).To(Equal(`devl ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa true true
qe ssh://root@podman.test:2222/run/podman/podman.sock ~/.ssh/id_rsa false true
`))

			cmd = []string{"system", "connection", "list"}
			session = podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.Out).Should(Say("Name *URI *Identity *Default"))
		})

		It("failed default", func() {
			cmd := []string{"system", "connection", "default", "devl"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).ShouldNot(ExitCleanly())
			Expect(session.Err).Should(Say("destination is not defined"))
		})

		It("failed rename", func() {
			cmd := []string{"system", "connection", "rename", "devl", "QE"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).ShouldNot(ExitCleanly())
			Expect(session.Err).Should(Say("destination is not defined"))
		})

		It("empty list", func() {
			cmd := []string{"system", "connection", "list"}
			session := podmanTest.Podman(cmd)
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
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

			cmd = exec.Command(podmanTest.RemotePodmanBinary, systemConnectionListCmd...)
			lsSession, err := Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			lsSession.Wait(DefaultWaitTimeout)
			Expect(lsSession).Should(Exit(0))
			Expect(string(lsSession.Out.Contents())).To(Equal("QA " + uri.String() + " " + filepath.Join(u.HomeDir, ".ssh", "id_ed25519") + " true true\n"))
		})
	})
})
