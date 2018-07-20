package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pod start", func() {
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

	It("podman pod start bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "start", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman pod start single empty pod", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman pod start single pod by name", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", "foobar99"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pod start multiple pods", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()
		cid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"pod", "create", "--name", "foobar100"})
		session2.WaitWithDefaultTimeout()
		cid2 := session2.OutputToString()

		session = podmanTest.Podman([]string{"create", "--pod", "foobar100", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", cid1, cid2})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})

	It("podman pod start all pods", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"pod", "create", "--name", "foobar100"})
		session2.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"create", "--pod", "foobar100", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})

	It("podman pod start latest pod", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"pod", "create", "--name", "foobar100"})
		session2.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"create", "--pod", "foobar100", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", "--latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman pod start multiple pods with bogus", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--name", "foobar99"})
		session.WaitWithDefaultTimeout()
		cid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", cid1, "doesnotexist"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})
})
