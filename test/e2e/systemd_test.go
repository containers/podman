package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman systemd", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman run container with systemd PID1", func() {
		ctrName := "testSystemd"
		run := podmanTest.Podman([]string{"run", "--name", ctrName, "-t", "-i", "-d", SYSTEMD_IMAGE, "/sbin/init"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))

		logs := podmanTest.Podman([]string{"logs", ctrName})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(Exit(0))

		// Give container 10 seconds to start
		started := podmanTest.WaitContainerReady(ctrName, "Reached target multi-user.target - Multi-User System.", 30, 1)
		Expect(started).To(BeTrue())

		systemctl := podmanTest.Podman([]string{"exec", "-t", "-i", ctrName, "systemctl", "status", "--no-pager"})
		systemctl.WaitWithDefaultTimeout()
		Expect(systemctl).Should(Exit(0))
		Expect(systemctl.OutputToString()).To(ContainSubstring("State:"))

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		conData := result.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].Config.SystemdMode).To(BeTrue())

		// stats not supported w/ CGv1 rootless or containerized
		if isCgroupsV1() && (isRootless() || isContainerized()) {
			return
		}
		stats := podmanTest.Podman([]string{"stats", "--no-stream", ctrName})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))

		// rootless with cgroupfs creates init.scope, skip cgroup path check
		if isRootless() && podmanTest.CgroupManager == "cgroupfs" {
			return
		}
		cgroupPath := podmanTest.Podman([]string{"inspect", "--format='{{.State.CgroupPath}}'", ctrName})
		cgroupPath.WaitWithDefaultTimeout()
		Expect(cgroupPath).Should(Exit(0))
		Expect(cgroupPath.OutputToString()).To(Not(ContainSubstring("init.scope")))
	})

	It("podman create container with systemd entrypoint triggers systemd mode", func() {
		ctrName := "testCtr"
		run := podmanTest.Podman([]string{"create", "--name", ctrName, "--entrypoint", "/sbin/init", SYSTEMD_IMAGE})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		conData := result.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].Config.SystemdMode).To(BeTrue())
	})

	It("podman systemd in command triggers systemd mode", func() {
		containerfile := fmt.Sprintf(`FROM %s
RUN mkdir -p /usr/lib/systemd/; touch /usr/lib/systemd/systemd
CMD /usr/lib/systemd/systemd`, ALPINE)

		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := os.WriteFile(containerfilePath, []byte(containerfile), 0755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "-t", "systemd", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		ctrName := "testCtr"
		run := podmanTest.Podman([]string{"create", "--name", ctrName, "systemd"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		conData := result.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].Config.SystemdMode).To(BeTrue())
	})

	It("podman create container with --uidmap and conmon PidFile accessible", func() {
		ctrName := "testCtrUidMap"
		run := podmanTest.Podman([]string{"run", "-d", "--uidmap=0:1:1000", "--name", ctrName, ALPINE, "top"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))

		session := podmanTest.Podman([]string{"inspect", "--format", "{{.ConmonPidFile}}", ctrName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		pidFile := strings.TrimSuffix(session.OutputToString(), "\n")
		_, err := os.ReadFile(pidFile)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman create container with systemd=always triggers systemd mode", func() {
		ctrName := "testCtr"
		run := podmanTest.Podman([]string{"create", "--name", ctrName, "--systemd", "always", ALPINE})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		conData := result.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].Config.SystemdMode).To(BeTrue())
	})

	It("podman run --systemd container should NOT mount /run noexec", func() {
		session := podmanTest.Podman([]string{"run", "--systemd", "always", ALPINE, "sh", "-c", "mount  | grep \"/run \""})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		Expect(session.OutputToString()).To(Not(ContainSubstring("noexec")))
	})

	It("podman run --systemd arg is case insensitive", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--systemd", "Always", ALPINE, "echo", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(Equal("test"))

		session = podmanTest.Podman([]string{"run", "--rm", "--systemd", "True", ALPINE, "echo", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(Equal("test"))

		session = podmanTest.Podman([]string{"run", "--rm", "--systemd", "False", ALPINE, "echo", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(Equal("test"))
	})
})
