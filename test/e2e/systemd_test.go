package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman systemd", func() {

	var systemdUnitFile string

	BeforeEach(func() {
		podmanCmd := fmt.Sprintf("%s %s", podmanTest.PodmanBinary, strings.Join(podmanTest.MakeOptions(nil, false, false), " "))
		systemdUnitFile = fmt.Sprintf(`[Unit]
Description=redis container
[Service]
Restart=always
ExecStart=%s start -a redis
ExecStop=%s stop -t 10 redis
KillMode=process
[Install]
WantedBy=default.target
`, podmanCmd, podmanCmd)
	})

	It("podman start container by systemd", func() {
		SkipIfRemote("cannot create unit file on remote host")
		SkipIfContainerized("test does not have systemd as pid 1")

		dashWhat := "--system"
		unitDir := "/run/systemd/system"
		if isRootless() {
			dashWhat = "--user"
			unitDir = fmt.Sprintf("%s/systemd/user", os.Getenv("XDG_RUNTIME_DIR"))
		}
		err := os.MkdirAll(unitDir, 0700)
		Expect(err).ToNot(HaveOccurred())

		serviceName := "redis-" + RandomString(10)
		sysFilePath := filepath.Join(unitDir, serviceName+".service")
		sysFile := os.WriteFile(sysFilePath, []byte(systemdUnitFile), 0644)
		Expect(sysFile).ToNot(HaveOccurred())
		defer func() {
			stop := SystemExec("systemctl", []string{dashWhat, "stop", serviceName})
			os.Remove(sysFilePath)
			SystemExec("systemctl", []string{dashWhat, "daemon-reload"})
			Expect(stop).Should(ExitCleanly())
		}()

		create := podmanTest.Podman([]string{"create", "--name", "redis", REDIS_IMAGE})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		enable := SystemExec("systemctl", []string{dashWhat, "daemon-reload"})
		Expect(enable).Should(ExitCleanly())

		start := SystemExec("systemctl", []string{dashWhat, "start", serviceName})
		Expect(start).Should(ExitCleanly())

		checkAvailableJournald()
		if !journald.journaldSkip {
			logs := SystemExec("journalctl", []string{dashWhat, "-n", "20", "-u", serviceName})
			Expect(logs).Should(ExitCleanly())
		}

		status := SystemExec("systemctl", []string{dashWhat, "status", serviceName})
		Expect(status.OutputToString()).To(ContainSubstring("active (running)"))
	})

	It("podman run container with systemd PID1", func() {
		ctrName := "testSystemd"
		run := podmanTest.Podman([]string{"run", "--name", ctrName, "-t", "-i", "-d", SYSTEMD_IMAGE, "/sbin/init"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))

		logs := podmanTest.Podman([]string{"logs", ctrName})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(ExitCleanly())

		// Give container 10 seconds to start
		started := podmanTest.WaitContainerReady(ctrName, "Reached target multi-user.target - Multi-User System.", 30, 1)
		Expect(started).To(BeTrue(), "Reached multi-user.target")

		systemctl := podmanTest.Podman([]string{"exec", ctrName, "systemctl", "status", "--no-pager"})
		systemctl.WaitWithDefaultTimeout()
		Expect(systemctl).Should(ExitCleanly())
		Expect(systemctl.OutputToString()).To(ContainSubstring("State:"))

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].Config).To(HaveField("SystemdMode", true))

		// stats not supported w/ CGv1 rootless or containerized
		if isCgroupsV1() && (isRootless() || isContainerized()) {
			return
		}
		stats := podmanTest.Podman([]string{"stats", "--no-stream", ctrName})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())

		cgroupPath := podmanTest.Podman([]string{"inspect", "--format='{{.State.CgroupPath}}'", ctrName})
		cgroupPath.WaitWithDefaultTimeout()
		Expect(cgroupPath).Should(ExitCleanly())
		Expect(cgroupPath.OutputToString()).To(Not(ContainSubstring("init.scope")))
	})

	It("podman create container with systemd entrypoint triggers systemd mode", func() {
		ctrName := "testCtr"
		run := podmanTest.Podman([]string{"create", "--name", ctrName, "--entrypoint", "/sbin/init", SYSTEMD_IMAGE})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].Config).To(HaveField("SystemdMode", true))
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
		Expect(session).Should(ExitCleanly())

		ctrName := "testCtr"
		run := podmanTest.Podman([]string{"create", "--name", ctrName, "systemd"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].Config).To(HaveField("SystemdMode", true))
	})

	It("podman create container with --uidmap and conmon PidFile accessible", func() {
		ctrName := "testCtrUidMap"
		run := podmanTest.Podman([]string{"run", "-d", "--uidmap=0:1:1000", "--name", ctrName, ALPINE, "top"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"inspect", "--format", "{{.ConmonPidFile}}", ctrName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		pidFile := strings.TrimSuffix(session.OutputToString(), "\n")
		_, err := os.ReadFile(pidFile)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman create container with systemd=always triggers systemd mode", func() {
		ctrName := "testCtr"
		run := podmanTest.Podman([]string{"create", "--name", ctrName, "--systemd", "always", ALPINE})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].Config).To(HaveField("SystemdMode", true))
	})

	It("podman run --systemd container should NOT mount /run noexec", func() {
		session := podmanTest.Podman([]string{"run", "--systemd", "always", ALPINE, "sh", "-c", "mount  | grep \"/run \""})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		Expect(session.OutputToString()).To(Not(ContainSubstring("noexec")))
	})

	It("podman run --systemd arg is case insensitive", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--systemd", "Always", ALPINE, "echo", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("test"))

		session = podmanTest.Podman([]string{"run", "--rm", "--systemd", "True", ALPINE, "echo", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("test"))

		session = podmanTest.Podman([]string{"run", "--rm", "--systemd", "False", ALPINE, "echo", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("test"))
	})
})
