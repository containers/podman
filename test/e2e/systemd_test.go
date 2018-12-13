package integration

import (
	"fmt"
	"io/ioutil"
	"os"

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
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
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
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman start container by systemd", func() {
		if os.Getenv("SKIP_USERNS") != "" {
			Skip("Skip userns tests.")
		}

		sys_file := ioutil.WriteFile("/etc/systemd/system/redis.service", []byte(systemd_unit_file), 0644)
		Expect(sys_file).To(BeNil())

		create := podmanTest.Podman([]string{"create", "-d", "--name", "redis", "redis"})
		create.WaitWithDefaultTimeout()
		Expect(create.ExitCode()).To(Equal(0))

		enable := SystemExec("bash", []string{"-c", "systemctl daemon-reload && systemctl enable --now redis"})
		enable.WaitWithDefaultTimeout()
		Expect(enable.ExitCode()).To(Equal(0))

		start := SystemExec("bash", []string{"-c", "systemctl start redis"})
		start.WaitWithDefaultTimeout()

		logs := SystemExec("bash", []string{"-c", "journalctl -n 20 -u redis"})
		logs.WaitWithDefaultTimeout()

		status := SystemExec("bash", []string{"-c", "systemctl status redis"})
		status.WaitWithDefaultTimeout()
		Expect(status.OutputToString()).To(ContainSubstring("active (running)"))

		cleanup := SystemExec("bash", []string{"-c", "systemctl stop redis && systemctl disable redis"})
		cleanup.WaitWithDefaultTimeout()
		Expect(cleanup.ExitCode()).To(Equal(0))
		os.Remove("/etc/systemd/system/redis.service")
		sys_clean := SystemExec("bash", []string{"-c", "systemctl daemon-reload"})
		sys_clean.WaitWithDefaultTimeout()
		Expect(sys_clean.ExitCode()).To(Equal(0))
	})
})
