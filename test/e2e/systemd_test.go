package integration

import (
	"io/ioutil"
	"os"
	"strings"
	"time"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman systemd", func() {
	var (
		tempdir         string
		err             error
		podmanTest      *PodmanTestIntegration
		systemdUnitFile string
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()
		systemdUnitFile = `[Unit]
Description=redis container
[Service]
Restart=always
ExecStart=/usr/bin/podman start -a redis
ExecStop=/usr/bin/podman stop -t 10 redis
KillMode=process
[Install]
WantedBy=multi-user.target
`
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman start container by systemd", func() {
		SkipIfRootless("rootless can not write to /etc")
		SkipIfContainerized("test does not have systemd as pid 1")

		sys_file := ioutil.WriteFile("/etc/systemd/system/redis.service", []byte(systemdUnitFile), 0644)
		Expect(sys_file).To(BeNil())
		defer func() {
			stop := SystemExec("bash", []string{"-c", "systemctl stop redis"})
			os.Remove("/etc/systemd/system/redis.service")
			SystemExec("bash", []string{"-c", "systemctl daemon-reload"})
			Expect(stop.ExitCode()).To(Equal(0))
		}()

		create := podmanTest.Podman([]string{"create", "--name", "redis", redis})
		create.WaitWithDefaultTimeout()
		Expect(create.ExitCode()).To(Equal(0))

		enable := SystemExec("bash", []string{"-c", "systemctl daemon-reload"})
		Expect(enable.ExitCode()).To(Equal(0))

		start := SystemExec("bash", []string{"-c", "systemctl start redis"})
		Expect(start.ExitCode()).To(Equal(0))

		logs := SystemExec("bash", []string{"-c", "journalctl -n 20 -u redis"})
		Expect(logs.ExitCode()).To(Equal(0))

		status := SystemExec("bash", []string{"-c", "systemctl status redis"})
		Expect(status.OutputToString()).To(ContainSubstring("active (running)"))
	})

	It("podman run container with systemd PID1", func() {
		ctrName := "testSystemd"
		run := podmanTest.Podman([]string{"run", "--name", ctrName, "-t", "-i", "-d", ubi_init, "/sbin/init"})
		run.WaitWithDefaultTimeout()
		Expect(run.ExitCode()).To(Equal(0))
		ctrID := run.OutputToString()

		logs := podmanTest.Podman([]string{"logs", ctrName})
		logs.WaitWithDefaultTimeout()
		Expect(logs.ExitCode()).To(Equal(0))

		// Give container 10 seconds to start
		started := false
		for i := 0; i < 10; i++ {
			runningCtrs := podmanTest.Podman([]string{"ps", "-q", "--no-trunc"})
			runningCtrs.WaitWithDefaultTimeout()
			Expect(runningCtrs.ExitCode()).To(Equal(0))

			if strings.Contains(runningCtrs.OutputToString(), ctrID) {
				started = true
				break
			}

			time.Sleep(1 * time.Second)
		}

		Expect(started).To(BeTrue())

		systemctl := podmanTest.Podman([]string{"exec", "-t", "-i", ctrName, "systemctl", "status", "--no-pager"})
		systemctl.WaitWithDefaultTimeout()
		Expect(systemctl.ExitCode()).To(Equal(0))
		Expect(strings.Contains(systemctl.OutputToString(), "State:")).To(BeTrue())

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		conData := result.InspectContainerToJSON()
		Expect(len(conData)).To(Equal(1))
		Expect(conData[0].Config.SystemdMode).To(BeTrue())
	})

	It("podman create container with systemd entrypoint triggers systemd mode", func() {
		ctrName := "testCtr"
		run := podmanTest.Podman([]string{"create", "--name", ctrName, "--entrypoint", "/sbin/init", ubi_init})
		run.WaitWithDefaultTimeout()
		Expect(run.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		conData := result.InspectContainerToJSON()
		Expect(len(conData)).To(Equal(1))
		Expect(conData[0].Config.SystemdMode).To(BeTrue())
	})

	It("podman create container with --uidmap and conmon PidFile accessible", func() {
		ctrName := "testCtrUidMap"
		run := podmanTest.Podman([]string{"run", "-d", "--uidmap=0:1:1000", "--name", ctrName, ALPINE, "top"})
		run.WaitWithDefaultTimeout()
		Expect(run.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"inspect", "--format", "{{.ConmonPidFile}}", ctrName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		pidFile := strings.TrimSuffix(session.OutputToString(), "\n")
		_, err := ioutil.ReadFile(pidFile)
		Expect(err).To(BeNil())
	})

	It("podman create container with systemd=always triggers systemd mode", func() {
		ctrName := "testCtr"
		run := podmanTest.Podman([]string{"create", "--name", ctrName, "--systemd", "always", ALPINE})
		run.WaitWithDefaultTimeout()
		Expect(run.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		conData := result.InspectContainerToJSON()
		Expect(len(conData)).To(Equal(1))
		Expect(conData[0].Config.SystemdMode).To(BeTrue())
	})

	It("podman run --systemd container should NOT mount /run noexec", func() {
		session := podmanTest.Podman([]string{"run", "--systemd", "always", ALPINE, "sh", "-c", "mount  | grep \"/run \""})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		Expect(session.OutputToString()).To(Not(ContainSubstring("noexec")))
	})
})
