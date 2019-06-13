// +build !remoteclient

package integration

import (
	"fmt"
	"os"
	"strings"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman port", func() {
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
		processTestResult(f)

	})

	It("podman port all and latest", func() {
		result := podmanTest.Podman([]string{"port", "-a", "-l"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))
	})

	It("podman port all and extra", func() {
		result := podmanTest.Podman([]string{"port", "-a", "foobar"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))
	})

	It("podman port -l nginx", func() {
		session, cid := podmanTest.RunNginxWithHealthCheck("")
		Expect(session.ExitCode()).To(Equal(0))

		if err := podmanTest.RunHealthCheck(cid); err != nil {
			Fail(err.Error())
		}

		result := podmanTest.Podman([]string{"port", "-l"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		port := strings.Split(result.OutputToStringArray()[0], ":")[1]
		Expect(result.LineInOuputStartsWith(fmt.Sprintf("80/tcp -> 0.0.0.0:%s", port))).To(BeTrue())
	})

	It("podman container port  -l nginx", func() {
		session, cid := podmanTest.RunNginxWithHealthCheck("")
		Expect(session.ExitCode()).To(Equal(0))

		if err := podmanTest.RunHealthCheck(cid); err != nil {
			Fail(err.Error())
		}

		result := podmanTest.Podman([]string{"container", "port", "-l"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		port := strings.Split(result.OutputToStringArray()[0], ":")[1]
		Expect(result.LineInOuputStartsWith(fmt.Sprintf("80/tcp -> 0.0.0.0:%s", port))).To(BeTrue())
	})

	It("podman port -l port nginx", func() {
		session, cid := podmanTest.RunNginxWithHealthCheck("")
		Expect(session.ExitCode()).To(Equal(0))

		if err := podmanTest.RunHealthCheck(cid); err != nil {
			Fail(err.Error())
		}

		result := podmanTest.Podman([]string{"port", "-l", "80"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		port := strings.Split(result.OutputToStringArray()[0], ":")[1]
		Expect(result.LineInOuputStartsWith(fmt.Sprintf("0.0.0.0:%s", port))).To(BeTrue())
	})

	It("podman port -a nginx", func() {
		session, cid := podmanTest.RunNginxWithHealthCheck("")
		Expect(session.ExitCode()).To(Equal(0))

		if err := podmanTest.RunHealthCheck(cid); err != nil {
			Fail(err.Error())
		}

		result := podmanTest.Podman([]string{"port", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})
})
