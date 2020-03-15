// +build !remoteclient

package integration

import (
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/containers/libpod/pkg/cgroups"
	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman systemd", func() {
	var (
		tempdir           string
		err               error
		podmanTest        *PodmanTestIntegration
		systemd_unit_file string
	)

	BeforeEach(func() {
		SkipIfRootless()
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()
		systemd_unit_file = `[Unit]
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
		if os.Getenv("SKIP_USERNS") != "" {
			Skip("Skip userns tests.")
		}

		sys_file := ioutil.WriteFile("/etc/systemd/system/redis.service", []byte(systemd_unit_file), 0644)
		Expect(sys_file).To(BeNil())
		defer func() {
			stop := SystemExec("bash", []string{"-c", "systemctl stop redis"})
			os.Remove("/etc/systemd/system/redis.service")
			SystemExec("bash", []string{"-c", "systemctl daemon-reload"})
			Expect(stop.ExitCode()).To(Equal(0))
		}()

		create := podmanTest.Podman([]string{"create", "-d", "--name", "redis", "redis"})
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
		cgroupsv2, err := cgroups.IsCgroup2UnifiedMode()
		Expect(err).To(BeNil())
		if cgroupsv2 {
			Skip("systemd test does not work in cgroups V2 mode yet")
		}

		systemdImage := "fedora"
		pull := podmanTest.Podman([]string{"pull", systemdImage})
		pull.WaitWithDefaultTimeout()
		Expect(pull.ExitCode()).To(Equal(0))

		ctrName := "testSystemd"
		run := podmanTest.Podman([]string{"run", "--name", ctrName, "-t", "-i", "-d", systemdImage, "/usr/sbin/init"})
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
	})
})
