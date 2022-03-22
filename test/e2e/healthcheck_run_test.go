package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	define "github.com/containers/podman/v4/libpod/define"
	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman healthcheck run", func() {
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
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))

	})

	It("podman healthcheck run bogus container", func() {
		session := podmanTest.Podman([]string{"healthcheck", "run", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman disable healthcheck with --no-healthcheck on valid container", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--no-healthcheck", "--name", "hc", healthcheck})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(125))
	})

	It("podman disable healthcheck with --no-healthcheck must not show starting on status", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--no-healthcheck", "--name", "hc", healthcheck})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		hc := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.State.Health.Status}}", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(0))
		Expect(hc.OutputToString()).To(Not(ContainSubstring("starting")))
	})

	It("podman run healthcheck and logs should contain healthcheck output", func() {
		session := podmanTest.Podman([]string{"run", "--name", "test-logs", "-dt", "--health-interval", "1s", "--health-cmd", "echo working", "busybox", "sleep", "3600"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Buy a little time to get container running
		for i := 0; i < 5; i++ {
			hc := podmanTest.Podman([]string{"healthcheck", "run", "test-logs"})
			hc.WaitWithDefaultTimeout()
			exitCode := hc.ExitCode()
			if exitCode == 0 || i == 4 {
				break
			}
			time.Sleep(1 * time.Second)
		}

		hc := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.State.Healthcheck.Log}}", "test-logs"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(0))
		Expect(hc.OutputToString()).To(ContainSubstring("working"))
	})

	It("podman healthcheck from image's config (not container config)", func() {
		// Regression test for #12226: a health check may be defined in
		// the container or the container-config of an image.
		session := podmanTest.Podman([]string{"create", "--name", "hc", "quay.io/libpod/healthcheck:config-only", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		hc := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Config.Healthcheck}}", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(0))
		Expect(hc.OutputToString()).To(Equal("{[CMD-SHELL curl -f http://localhost/ || exit 1] 0s 5m0s 3s 0}"))
	})

	It("podman disable healthcheck with --health-cmd=none on valid container", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--health-cmd", "none", "--name", "hc", healthcheck})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(125))
	})

	It("podman healthcheck on valid container", func() {
		Skip("Extremely consistent flake - re-enable on debugging")
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", healthcheck})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		exitCode := 999

		// Buy a little time to get container running
		for i := 0; i < 5; i++ {
			hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
			hc.WaitWithDefaultTimeout()
			exitCode = hc.ExitCode()
			if exitCode == 0 || i == 4 {
				break
			}
			time.Sleep(1 * time.Second)
		}
		Expect(exitCode).To(Equal(0))

		ps := podmanTest.Podman([]string{"ps"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(Exit(0))
		Expect(ps.OutputToString()).To(ContainSubstring("(healthy)"))
	})

	It("podman healthcheck that should fail", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "quay.io/libpod/badhealthcheck:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))
	})

	It("podman healthcheck on stopped container", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", healthcheck, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(125))
	})

	It("podman healthcheck on container without healthcheck", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(125))
	})

	It("podman healthcheck should be starting", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-retries", "2", "--health-cmd", "ls /foo || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health.Status).To(Equal("starting"))
	})

	It("podman healthcheck failed checks in start-period should not change status", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-start-period", "2m", "--health-retries", "2", "--health-cmd", "ls /foo || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health.Status).To(Equal("starting"))
		// test old podman compat (see #11645)
		Expect(inspect[0].State.Healthcheck().Status).To(Equal("starting"))
	})

	It("podman healthcheck failed checks must reach retries before unhealthy ", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-retries", "2", "--health-cmd", "ls /foo || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health.Status).To(Equal("starting"))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		inspect = podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health.Status).To(Equal(define.HealthCheckUnhealthy))
		// test old podman compat (see #11645)
		Expect(inspect[0].State.Healthcheck().Status).To(Equal(define.HealthCheckUnhealthy))
	})

	It("podman healthcheck good check results in healthy even in start-period", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-start-period", "2m", "--health-retries", "2", "--health-cmd", "ls || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(0))

		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health.Status).To(Equal(define.HealthCheckHealthy))
	})

	It("podman healthcheck unhealthy but valid arguments check", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-retries", "2", "--health-cmd", "[\"ls\", \"/foo\"]", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))
	})

	It("podman healthcheck single healthy result changes failed to healthy", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-retries", "2", "--health-cmd", "ls /foo || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health.Status).To(Equal("starting"))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		inspect = podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health.Status).To(Equal(define.HealthCheckUnhealthy))

		foo := podmanTest.Podman([]string{"exec", "hc", "touch", "/foo"})
		foo.WaitWithDefaultTimeout()
		Expect(foo).Should(Exit(0))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(0))

		inspect = podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health.Status).To(Equal(define.HealthCheckHealthy))

		// Test podman ps --filter heath is working (#11687)
		ps := podmanTest.Podman([]string{"ps", "--filter", "health=healthy"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(Exit(0))
		Expect(ps.OutputToStringArray()).To(HaveLen(2))
		Expect(ps.OutputToString()).To(ContainSubstring("hc"))
	})

	It("stopping and then starting a container with healthcheck cmd", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-cmd", "[\"ls\", \"/foo\"]", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		stop := podmanTest.Podman([]string{"stop", "-t0", "hc"})
		stop.WaitWithDefaultTimeout()
		Expect(stop).Should(Exit(0))

		startAgain := podmanTest.Podman([]string{"start", "hc"})
		startAgain.WaitWithDefaultTimeout()
		Expect(startAgain).Should(Exit(0))
		Expect(startAgain.OutputToString()).To(Equal("hc"))
		Expect(startAgain.ErrorToString()).To(Equal(""))
	})

	It("Verify default time is used and no utf-8 escapes", func() {
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())

		podmanTest.AddImageToRWStore(ALPINE)
		// Write target and fake files
		targetPath, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())
		containerfile := fmt.Sprintf(`FROM %s
HEALTHCHECK CMD ls -l / 2>&1`, ALPINE)
		containerfilePath := filepath.Join(targetPath, "Containerfile")
		err = ioutil.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).To(BeNil())
		defer func() {
			Expect(os.Chdir(cwd)).To(BeNil())
			Expect(os.RemoveAll(targetPath)).To(BeNil())
		}()

		// make cwd as context root path
		Expect(os.Chdir(targetPath)).To(BeNil())

		session := podmanTest.Podman([]string{"build", "--format", "docker", "-t", "test", "."})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		run := podmanTest.Podman([]string{"run", "-dt", "--name", "hctest", "test", "ls"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))

		inspect := podmanTest.InspectContainer("hctest")
		// Check to make sure a default time value was added
		Expect(inspect[0].Config.Healthcheck.Timeout).To(BeNumerically("==", 30000000000))
		// Check to make sure characters were not coerced to utf8
		Expect(inspect[0].Config.Healthcheck.Test).To(Equal([]string{"CMD-SHELL", "ls -l / 2>&1"}))
	})
})
