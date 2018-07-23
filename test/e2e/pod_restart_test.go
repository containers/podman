package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pod restart", func() {
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
	})

	It("podman pod restart bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "restart", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman pod restart single empty pod", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "restart", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman pod restart single pod by name", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("test1", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		startTime.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"pod", "restart", "foobar99"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToString()).To(Not(Equal(startTime.OutputToString())))
	})

	It("podman pod restart multiple pods", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()

		session = podmanTest.RunTopContainerInPod("test1", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"pod", "create", "--name", "foobar100"})
		session2.WaitWithDefaultTimeout()

		session = podmanTest.RunTopContainerInPod("test2", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"pod", "restart", "foobar99", "foobar100"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Not(Equal(startTime.OutputToStringArray()[0])))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("podman pod restart all pods", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()

		session = podmanTest.RunTopContainerInPod("test1", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"pod", "create", "--name", "foobar100"})
		session2.WaitWithDefaultTimeout()

		session = podmanTest.RunTopContainerInPod("test2", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"pod", "restart", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Not(Equal(startTime.OutputToStringArray()[0])))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("podman pod restart latest pod", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()

		session = podmanTest.RunTopContainerInPod("test1", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"pod", "create", "--name", "foobar100"})
		session2.WaitWithDefaultTimeout()

		session = podmanTest.RunTopContainerInPod("test2", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"pod", "restart", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Equal(startTime.OutputToStringArray()[0]))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("podman pod restart multiple pods with bogus", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()
		cid1 := session.OutputToString()

		session = podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "restart", cid1, "doesnotexist"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})
})
