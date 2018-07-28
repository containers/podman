package integration

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman restart", func() {
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

	It("Podman restart bogus container", func() {
		session := podmanTest.Podman([]string{"start", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("Podman restart stopped container by name", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))
		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToString()).To(Not(Equal(startTime.OutputToString())))
	})

	It("Podman restart stopped container by ID", func() {
		session := podmanTest.Podman([]string{"create", "-d", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()
		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", cid})
		startTime.WaitWithDefaultTimeout()

		startSession := podmanTest.Podman([]string{"start", cid})
		startSession.WaitWithDefaultTimeout()
		Expect(startSession.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"restart", cid})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", cid})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToString()).To(Not(Equal(startTime.OutputToString())))
	})

	It("Podman restart running container", func() {
		_ = podmanTest.RunTopContainer("test1")
		ok := WaitForContainer(&podmanTest)
		Expect(ok).To(BeTrue())
		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToString()).To(Not(Equal(startTime.OutputToString())))
	})

	It("Podman restart multiple containers", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))

		_, exitCode, _ = podmanTest.RunLsContainer("test2")
		Expect(exitCode).To(Equal(0))
		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "test1", "test2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Not(Equal(startTime.OutputToStringArray()[0])))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("Podman restart the latest container", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))

		_, exitCode, _ = podmanTest.RunLsContainer("test2")
		Expect(exitCode).To(Equal(0))

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Equal(startTime.OutputToStringArray()[0]))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("Podman restart non-stop container with short timeout", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test1", "--env", "STOPSIGNAL=SIGKILL", ALPINE, "sleep", "999"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		startTime := time.Now()
		session = podmanTest.Podman([]string{"restart", "-t", "2", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		timeSince := time.Since(startTime)
		Expect(timeSince < 10*time.Second).To(BeTrue())
		Expect(timeSince > 2*time.Second).To(BeTrue())
	})
})
