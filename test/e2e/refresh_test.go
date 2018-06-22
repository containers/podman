package integration

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman refresh", func() {
	var (
		tmpdir     string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tmpdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tmpdir)
		podmanTest.RestoreAllArtifacts()
		podmanTest.RestoreArtifact(fedoraMinimal)
	})

	AfterEach(func() {
		podmanTest.Cleanup()
	})

	Specify("Refresh with no containers succeeds", func() {
		session := podmanTest.Podman([]string{"container", "refresh"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	Specify("Refresh with created container succeeds", func() {
		createSession := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		createSession.WaitWithDefaultTimeout()
		Expect(createSession.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.NumberOfRunningContainers()).To(Equal(0))

		refreshSession := podmanTest.Podman([]string{"container", "refresh"})
		refreshSession.WaitWithDefaultTimeout()
		Expect(refreshSession.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.NumberOfRunningContainers()).To(Equal(0))
	})

	Specify("Refresh with running container restarts container", func() {
		createSession := podmanTest.Podman([]string{"run", "-d", ALPINE, "sleep", "120"})
		createSession.WaitWithDefaultTimeout()
		Expect(createSession.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.NumberOfRunningContainers()).To(Equal(1))

		// HACK: ensure container starts before we move on
		time.Sleep(1 * time.Second)

		refreshSession := podmanTest.Podman([]string{"container", "refresh"})
		refreshSession.WaitWithDefaultTimeout()
		Expect(refreshSession.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.NumberOfRunningContainers()).To(Equal(1))
	})
})
