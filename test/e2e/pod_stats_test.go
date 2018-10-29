package integration

import (
	"fmt"
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pod stats", func() {
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
		podmanTest.CleanupPod()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})
	It("podman stats should run with no pods", func() {
		session := podmanTest.Podman([]string{"pod", "stats", "--no-stream"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman stats with a bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "stats", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman stats on a specific running pod", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream", podid})
		stats.WaitWithDefaultTimeout()
		Expect(stats.ExitCode()).To(Equal(0))
	})

	It("podman stats on a specific running pod with shortID", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream", podid[:5]})
		stats.WaitWithDefaultTimeout()
		Expect(stats.ExitCode()).To(Equal(0))
	})

	It("podman stats on a specific running pod with name", func() {
		_, ec, podid := podmanTest.CreatePod("test")
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream", "test"})
		stats.WaitWithDefaultTimeout()
		Expect(stats.ExitCode()).To(Equal(0))
	})

	It("podman stats on running pods", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream"})
		stats.WaitWithDefaultTimeout()
		Expect(stats.ExitCode()).To(Equal(0))
	})

	It("podman stats on all pods", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream", "-a"})
		stats.WaitWithDefaultTimeout()
		Expect(stats.ExitCode()).To(Equal(0))
	})

	It("podman stats with json output", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--format", "json", "--no-stream", "-a"})
		stats.WaitWithDefaultTimeout()
		Expect(stats.ExitCode()).To(Equal(0))
		Expect(stats.IsJSONOutputValid()).To(BeTrue())
	})

})
