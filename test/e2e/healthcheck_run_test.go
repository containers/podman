package integration

import (
	"fmt"
	"os"
	"time"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		SkipIfRemote()
		session := podmanTest.Podman([]string{"run", "-dt", "--no-healthcheck", "--name", "hc", healthcheck})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(125))
	})

	It("podman disable healthcheck with --health-cmd=none on valid container", func() {
		SkipIfRemote()
		session := podmanTest.Podman([]string{"run", "-dt", "--health-cmd", "none", "--name", "hc", healthcheck})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(125))
	})

	It("podman healthcheck on valid container", func() {
		Skip("Extremely consistent flake - re-enable on debugging")
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", healthcheck})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

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
	})

	It("podman healthcheck that should fail", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "docker.io/libpod/badhealthcheck:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(1))
	})

	It("podman healthcheck on stopped container", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", healthcheck, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(125))
	})

	It("podman healthcheck on container without healthcheck", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(125))
	})

	It("podman healthcheck should be starting", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-retries", "2", "--health-cmd", "ls /foo || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Healthcheck.Status).To(Equal("starting"))
	})

	It("podman healthcheck failed checks in start-period should not change status", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-start-period", "2m", "--health-retries", "2", "--health-cmd", "ls /foo || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(1))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(1))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(1))

		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Healthcheck.Status).To(Equal("starting"))
	})

	It("podman healthcheck failed checks must reach retries before unhealthy ", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-retries", "2", "--health-cmd", "ls /foo || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(1))

		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Healthcheck.Status).To(Equal("starting"))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(1))

		inspect = podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Healthcheck.Status).To(Equal("unhealthy"))

	})

	It("podman healthcheck good check results in healthy even in start-period", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-start-period", "2m", "--health-retries", "2", "--health-cmd", "ls || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(0))

		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Healthcheck.Status).To(Equal("healthy"))
	})

	It("podman healthcheck single healthy result changes failed to healthy", func() {
		Skip(v2remotefail)
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-retries", "2", "--health-cmd", "ls /foo || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(1))

		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Healthcheck.Status).To(Equal("starting"))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(1))

		inspect = podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Healthcheck.Status).To(Equal("unhealthy"))

		foo := podmanTest.Podman([]string{"exec", "hc", "touch", "/foo"})
		foo.WaitWithDefaultTimeout()
		Expect(foo.ExitCode()).To(BeZero())

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc.ExitCode()).To(Equal(0))

		inspect = podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Healthcheck.Status).To(Equal("healthy"))
	})
})
