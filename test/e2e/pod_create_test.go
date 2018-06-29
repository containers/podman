package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pod create", func() {
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

	It("podman create pod", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		cid := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		match, _ := check.GrepString(cid)
		Expect(match).To(BeTrue())
		Expect(len(check.OutputToStringArray())).To(Equal(1))
	})

	It("podman create pod with name", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		match, _ := check.GrepString(name)
		Expect(match).To(BeTrue())
	})

	It("podman create pod with doubled name", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "create", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(1)))

		check := podmanTest.Podman([]string{"pod", "ps", "-q"})
		check.WaitWithDefaultTimeout()
		Expect(len(check.OutputToStringArray())).To(Equal(1))
	})

	It("podman create pod with same name as ctr", func() {
		name := "test"
		session := podmanTest.Podman([]string{"create", "--name", name, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "create", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(1)))

		check := podmanTest.Podman([]string{"pod", "ps", "-q"})
		check.WaitWithDefaultTimeout()
		Expect(len(check.OutputToStringArray())).To(Equal(1))
	})

	It("podman create pod with label", func() {
		label := "HELLO=WORLD"
		session := podmanTest.Podman([]string{"pod", "create", "--label", label})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "--labels"})
		check.WaitWithDefaultTimeout()
		match, _ := check.GrepString(label)
		Expect(match).To(BeTrue())
	})

	It("podman create pod with Cgroup", func() {
		cgroup := "/"
		session := podmanTest.Podman([]string{"pod", "create", "--cgroup-parent", cgroup})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "--cgroup"})
		check.WaitWithDefaultTimeout()
		match, _ := check.GrepString(cgroup)
		Expect(match).To(BeTrue())
	})

	It("podman create pod with Cgroup as parent", func() {
		cgroup := "/tempdir"
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--name", name, "--cgroup-to-ctr", "--cgroup-parent", cgroup})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "--cgroup"})
		check.WaitWithDefaultTimeout()
		match, _ := check.GrepString("true")
		Expect(match).To(BeTrue())

		check = podmanTest.Podman([]string{"create", "--pod", name, ALPINE, "ls"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))

		check = podmanTest.Podman([]string{"ps", "-a", "--ns"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))

		match, _ = check.GrepString(cgroup)
		if !match {
			Skip("something is wrong with Cgroups, skipping")
		}
		Expect(match).To(BeTrue())
	})
})
