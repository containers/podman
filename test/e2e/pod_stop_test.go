package integration

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pod stop", func() {
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
		podmanTest.CleanupPod()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman pod stop bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "stop", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman pod stop single empty pod", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "stop", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pod stop single pod by name", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", "foobar99"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod stop multiple pods", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()
		cid1 := session.OutputToString()

		session = podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"pod", "create", "--name", "foobar100"})
		session2.WaitWithDefaultTimeout()
		cid2 := session2.OutputToString()

		session = podmanTest.RunTopContainerInPod("", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", cid1, cid2})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod stop all pods", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()

		session = podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"pod", "create", "--name", "foobar100"})
		session2.WaitWithDefaultTimeout()

		session = podmanTest.RunTopContainerInPod("", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod stop latest pod", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()

		session = podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"pod", "create", "--name", "foobar100"})
		session2.WaitWithDefaultTimeout()

		session = podmanTest.RunTopContainerInPod("", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", "--latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman pod stop multiple pods with bogus", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()
		cid1 := session.OutputToString()

		session = podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "stop", cid1, "doesnotexist"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})
})
