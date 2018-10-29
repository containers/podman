package integration

import (
	"fmt"
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman logs", func() {
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	//sudo bin/podman run -it --rm fedora-minimal bash -c 'for a in `seq 5`; do echo hello; done'
	It("podman logs for container", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("podman logs tail two lines", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--tail", "2", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(len(results.OutputToStringArray())).To(Equal(2))
	})

	It("podman logs tail 99 lines", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--tail", "99", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("podman logs tail 2 lines with timestamps", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--tail", "2", "-t", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(len(results.OutputToStringArray())).To(Equal(2))
	})

	It("podman logs latest with since time", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--since", "2017-08-07T10:10:09.056611202-04:00", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("podman logs latest with since duration", func() {
		logc := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--since", "10m", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})
})
