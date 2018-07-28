package integration

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run cpu", func() {
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

	It("podman run cpu-period", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-period=5000", ALPINE, "cat", "/sys/fs/cgroup/cpu/cpu.cfs_period_us"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("5000"))
	})

	It("podman run cpu-quota", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-quota=5000", ALPINE, "cat", "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("5000"))
	})

	It("podman run cpus", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpus=0.5", ALPINE, "cat", "/sys/fs/cgroup/cpu/cpu.cfs_period_us"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("100000"))

		result = podmanTest.Podman([]string{"run", "--rm", "--cpus=0.5", ALPINE, "cat", "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("50000"))
	})

	It("podman run cpu-shares", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-shares=2", ALPINE, "cat", "/sys/fs/cgroup/cpu/cpu.shares"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("2"))
	})

	It("podman run cpuset-cpus", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpuset-cpus=0", ALPINE, "cat", "/sys/fs/cgroup/cpuset/cpuset.cpus"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("0"))
	})

	It("podman run cpuset-mems", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpuset-mems=0", ALPINE, "cat", "/sys/fs/cgroup/cpuset/cpuset.mems"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(Equal("0"))
	})

	It("podman run cpus and cpu-period", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-period=5000", "--cpus=0.5", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Not(Equal(0)))
	})

	It("podman run cpus and cpu-quota", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-quota=5000", "--cpus=0.5", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Not(Equal(0)))
	})
})
