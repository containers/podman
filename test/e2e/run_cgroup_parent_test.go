package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run with --cgroup-parent", func() {
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
		podmanTest.RestoreArtifact(fedoraMinimal)
	})

	AfterEach(func() {
		podmanTest.Cleanup()

	})

	Specify("valid --cgroup-parent using cgroupfs", func() {
		cgroup := "/zzz"
		run := podmanTest.Podman([]string{"run", "--cgroup-parent", cgroup, fedoraMinimal, "cat", "/proc/self/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run.ExitCode()).To(Equal(0))
		ok, _ := run.GrepString(cgroup)
		Expect(ok).To(BeTrue())
	})

	Specify("no --cgroup-parent", func() {
		cgroup := "/libpod_parent"
		run := podmanTest.Podman([]string{"run", fedoraMinimal, "cat", "/proc/self/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run.ExitCode()).To(Equal(0))
		ok, _ := run.GrepString(cgroup)
		Expect(ok).To(BeTrue())
	})

	Specify("valid --cgroup-parent using slice", func() {
		cgroup := "aaaa.slice"
		run := podmanTest.Podman([]string{"run", "--cgroup-parent", cgroup, fedoraMinimal, "cat", "/proc/1/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run.ExitCode()).To(Equal(0))
		ok, _ := run.GrepString(cgroup)
		Expect(ok).To(BeTrue())
	})
})
