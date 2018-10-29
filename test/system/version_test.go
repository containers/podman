package system

import (
	"fmt"
	"os"
	"regexp"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman version test", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestSystem
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("Smoking test: podman version with extra args", func() {
		logc := podmanTest.Podman([]string{"version", "anything", "-", "--"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		ver := logc.OutputToString()
		Expect(regexp.MatchString("Version:.*?Go Version:.*?OS/Arch", ver)).To(BeTrue())
	})

	It("Negative test: podman version with extra flag", func() {
		logc := podmanTest.Podman([]string{"version", "--foo"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).NotTo(Equal(0))
		err, _ := logc.GrepString("Incorrect Usage: flag provided but not defined: -foo")
		Expect(err).To(BeTrue())
	})

})
