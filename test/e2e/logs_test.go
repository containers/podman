// +build !remoteclient

package integration

import (
	"os"
	"strings"

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
		podmanTest.Setup()
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

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

	It("podman logs latest and container name should fail", func() {
		results := podmanTest.Podman([]string{"logs", "-l", "foobar"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).ToNot(Equal(0))
	})

	It("podman logs two containers and should display short container IDs", func() {
		log1 := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		log1.WaitWithDefaultTimeout()
		Expect(log1.ExitCode()).To(Equal(0))
		cid1 := log1.OutputToString()

		log2 := podmanTest.Podman([]string{"run", "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
		log2.WaitWithDefaultTimeout()
		Expect(log2.ExitCode()).To(Equal(0))
		cid2 := log2.OutputToString()

		results := podmanTest.Podman([]string{"logs", cid1, cid2})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))

		output := results.OutputToStringArray()
		Expect(len(output)).To(Equal(6))
		Expect(strings.Contains(output[0], cid1[:12]) || strings.Contains(output[0], cid2[:12])).To(BeTrue())
	})

	It("podman logs on a created container should result in 0 exit code", func() {
		session := podmanTest.Podman([]string{"create", "-dt", "--name", "log", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())

		results := podmanTest.Podman([]string{"logs", "log"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(BeZero())
	})
})
