package integration

import (
	"os"
	"time"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman restart", func() {
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("Podman restart bogus container", func() {
		session := podmanTest.Podman([]string{"start", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("Podman restart stopped container by name", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))
		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToString()).To(Not(Equal(startTime.OutputToString())))
	})

	It("Podman restart stopped container by ID", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", cid})
		startTime.WaitWithDefaultTimeout()

		startSession := podmanTest.Podman([]string{"start", cid})
		startSession.WaitWithDefaultTimeout()
		Expect(startSession).Should(Exit(0))

		session2 := podmanTest.Podman([]string{"restart", cid})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", cid})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToString()).To(Not(Equal(startTime.OutputToString())))
	})

	It("Podman restart running container", func() {
		_ = podmanTest.RunTopContainer("test1")
		ok := WaitForContainer(podmanTest)
		Expect(ok).To(BeTrue())
		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToString()).To(Not(Equal(startTime.OutputToString())))
	})

	It("Podman container restart running container", func() {
		_ = podmanTest.RunTopContainer("test1")
		ok := WaitForContainer(podmanTest)
		Expect(ok).To(BeTrue())
		startTime := podmanTest.Podman([]string{"container", "inspect", "--format='{{.State.StartedAt}}'", "test1"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"container", "restart", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		restartTime := podmanTest.Podman([]string{"container", "inspect", "--format='{{.State.StartedAt}}'", "test1"})
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
		Expect(session).Should(Exit(0))
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

		cid := "-l"
		if IsRemote() {
			cid = "test2"
		}
		session := podmanTest.Podman([]string{"restart", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Equal(startTime.OutputToStringArray()[0]))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("Podman restart non-stop container with short timeout", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test1", "--env", "STOPSIGNAL=SIGKILL", ALPINE, "sleep", "999"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		startTime := time.Now()
		session = podmanTest.Podman([]string{"restart", "-t", "2", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		timeSince := time.Since(startTime)
		Expect(timeSince).To(BeNumerically("<", 10*time.Second))
		Expect(timeSince).To(BeNumerically(">", 2*time.Second))
	})

	It("Podman restart --all", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))

		test2 := podmanTest.RunTopContainer("test2")
		test2.WaitWithDefaultTimeout()
		Expect(test2).Should(Exit(0))

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Not(Equal(startTime.OutputToStringArray()[0])))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("Podman restart --all --running", func() {
		_, exitCode, _ := podmanTest.RunLsContainer("test1")
		Expect(exitCode).To(Equal(0))

		test2 := podmanTest.RunTopContainer("test2")
		test2.WaitWithDefaultTimeout()
		Expect(test2).Should(Exit(0))

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"restart", "-a", "--running"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Equal(startTime.OutputToStringArray()[0]))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("Podman restart a container in a pod and hosts should not duplicated", func() {
		// Fixes: https://github.com/containers/podman/issues/8921

		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("host-restart-test", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		testCmd := []string{"exec", "host-restart-test", "sh", "-c", "wc -l < /etc/hosts"}

		// before restart
		beforeRestart := podmanTest.Podman(testCmd)
		beforeRestart.WaitWithDefaultTimeout()
		Expect(beforeRestart).Should(Exit(0))

		session = podmanTest.Podman([]string{"restart", "host-restart-test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		afterRestart := podmanTest.Podman(testCmd)
		afterRestart.WaitWithDefaultTimeout()
		Expect(afterRestart).Should(Exit(0))

		// line count should be equal
		Expect(beforeRestart.OutputToString()).To(Equal(afterRestart.OutputToString()))
	})

	It("podman restart --all", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		session = podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))

		session = podmanTest.Podman([]string{"stop", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		session = podmanTest.Podman([]string{"restart", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})
})
