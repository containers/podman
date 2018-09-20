package integration

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman checkpoint", func() {
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
		// At least CRIU 3.11 is needed
		skip, err := podmanTest.isCriuAtLeast(31100)
		if err != nil || skip {
			Skip("CRIU missing or too old.")
		}
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman checkpoint bogus container", func() {
		session := podmanTest.Podman([]string{"container", "checkpoint", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman restore bogus container", func() {
		session := podmanTest.Podman([]string{"container", "restore", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman checkpoint a running container by id", func() {
		// CRIU does not work with seccomp correctly on RHEL7
		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", "seccomp=unconfined", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "checkpoint", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
	})

	It("podman checkpoint a running container by name", func() {
		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", "seccomp=unconfined", "--name", "test_name", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"container", "checkpoint", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
	})

	It("podman pause a checkpointed container by id", func() {
		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", "seccomp=unconfined", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "checkpoint", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", "-f", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

	})
})
