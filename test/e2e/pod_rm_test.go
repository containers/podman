package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pod rm", func() {
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

	It("podman pod rm empty pod", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		podid := session.OutputToString()

		result := podmanTest.Podman([]string{"pod", "rm", podid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman pod rm latest pod", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		podid := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		podid2 := session.OutputToString()

		result := podmanTest.Podman([]string{"pod", "rm", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(ContainSubstring(podid))
		Expect(result.OutputToString()).To(Not(ContainSubstring(podid2)))
	})

	It("podman pod rm doesn't remove a pod with a container", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()

		podid := session.OutputToString()

		session = podmanTest.Podman([]string{"create", "--pod", podid, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "rm", podid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))

		result = podmanTest.Podman([]string{"ps", "-qa"})
		result.WaitWithDefaultTimeout()
		Expect(len(result.OutputToStringArray())).To(Equal(1))
	})

	It("podman pod rm -f does remove a running container", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()

		podid := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "-d", "--pod", podid, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "rm", "-f", podid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(BeEmpty())
	})

	It("podman pod rm -a doesn't remove a running container", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()

		podid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"run", "-d", "--pod", podid1, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "rm", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Not(Equal(0)))

		result = podmanTest.Podman([]string{"ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(len(result.OutputToStringArray())).To(Equal(1))

		// one pod should have been deleted
		result = podmanTest.Podman([]string{"pod", "ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(len(result.OutputToStringArray())).To(Equal(1))
	})

	It("podman pod rm -fa removes everything", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()

		podid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()

		podid2 := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"run", "-d", "--pod", podid1, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "-d", "--pod", podid1, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "-d", "--pod", podid2, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "-d", "--pod", podid2, nginx})
		session.WaitWithDefaultTimeout()

		result := podmanTest.Podman([]string{"pod", "rm", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(BeEmpty())

		// one pod should have been deleted
		result = podmanTest.Podman([]string{"pod", "ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(BeEmpty())
	})
})
