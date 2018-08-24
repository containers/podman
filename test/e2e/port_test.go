package integration

import (
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman port", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
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

	It("podman port  -l nginx", func() {
		podmanTest.RestoreArtifact(nginx)
		session := podmanTest.Podman([]string{"run", "-dt", "-P", nginx})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"port", "-l"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		port := strings.Split(result.OutputToStringArray()[0], ":")[1]
		Expect(result.LineInOuputStartsWith(fmt.Sprintf("80/tcp -> 0.0.0.0:%s", port))).To(BeTrue())
	})

	It("podman port -l port nginx", func() {
		podmanTest.RestoreArtifact(nginx)
		session := podmanTest.Podman([]string{"run", "-dt", "-P", nginx})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"port", "-l", "80"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		port := strings.Split(result.OutputToStringArray()[0], ":")[1]
		Expect(result.LineInOuputStartsWith(fmt.Sprintf("0.0.0.0:%s", port))).To(BeTrue())
	})

	It("podman port -a nginx", func() {
		podmanTest.RestoreArtifact(nginx)
		session := podmanTest.Podman([]string{"run", "-dt", "-P", nginx})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"port", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})
})
