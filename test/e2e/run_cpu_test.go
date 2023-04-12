package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run cpu", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootlessCgroupsV1("Setting CPU not supported on cgroupv1 for rootless users")

		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}

		if CGROUPSV2 {
			if err := os.WriteFile("/sys/fs/cgroup/cgroup.subtree_control", []byte("+cpuset"), 0644); err != nil {
				Skip("cpuset controller not available on the current kernel")
			}
		}

		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman run cpu-period", func() {
		var result *PodmanSessionIntegration
		if CGROUPSV2 {
			result = podmanTest.Podman([]string{"run", "--rm", "--cpu-period=5000", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/cpu.max"})
		} else {
			result = podmanTest.Podman([]string{"run", "--rm", "--cpu-period=5000", ALPINE, "cat", "/sys/fs/cgroup/cpu/cpu.cfs_period_us"})
		}
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring("5000"))
	})

	It("podman run cpu-quota", func() {
		var result *PodmanSessionIntegration

		if CGROUPSV2 {
			result = podmanTest.Podman([]string{"run", "--rm", "--cpu-quota=5000", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/cpu.max"})
		} else {
			result = podmanTest.Podman([]string{"run", "--rm", "--cpu-quota=5000", ALPINE, "cat", "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"})
		}
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring("5000"))
	})

	It("podman run cpus", func() {
		if CGROUPSV2 {
			result := podmanTest.Podman([]string{"run", "--rm", "--cpu-quota=5000", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/cpu.max"})
			result.WaitWithDefaultTimeout()
			Expect(result).Should(Exit(0))
			Expect(result.OutputToString()).To(Equal("5000 100000"))
		} else {
			result := podmanTest.Podman([]string{"run", "--rm", "--cpus=0.5", ALPINE, "cat", "/sys/fs/cgroup/cpu/cpu.cfs_period_us"})
			result.WaitWithDefaultTimeout()
			Expect(result).Should(Exit(0))
			Expect(result.OutputToString()).To(Equal("100000"))

			result = podmanTest.Podman([]string{"run", "--rm", "--cpus=0.5", ALPINE, "cat", "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"})
			result.WaitWithDefaultTimeout()
			Expect(result).Should(Exit(0))
			Expect(result.OutputToString()).To(Equal("50000"))
		}
	})

	It("podman run cpu-shares", func() {
		if CGROUPSV2 {
			// [2-262144] is mapped to [1-10000]
			result := podmanTest.Podman([]string{"run", "--rm", "--cpu-shares=262144", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/cpu.weight"})
			result.WaitWithDefaultTimeout()
			Expect(result).Should(Exit(0))
			Expect(result.OutputToString()).To(Equal("10000"))
		} else {
			result := podmanTest.Podman([]string{"run", "--rm", "-c", "2", ALPINE, "cat", "/sys/fs/cgroup/cpu/cpu.shares"})
			result.WaitWithDefaultTimeout()
			Expect(result).Should(Exit(0))
			Expect(result.OutputToString()).To(Equal("2"))
		}
	})

	It("podman run cpuset-cpus", func() {
		var result *PodmanSessionIntegration

		if CGROUPSV2 {
			result = podmanTest.Podman([]string{"run", "--rm", "--cpuset-cpus=0", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/cpuset.cpus.effective"})
		} else {
			result = podmanTest.Podman([]string{"run", "--rm", "--cpuset-cpus=0", ALPINE, "cat", "/sys/fs/cgroup/cpuset/cpuset.cpus"})
		}
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(Equal("0"))
	})

	It("podman run cpuset-mems", func() {
		var result *PodmanSessionIntegration

		if CGROUPSV2 {
			result = podmanTest.Podman([]string{"run", "--rm", "--cpuset-mems=0", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/cpuset.mems.effective"})
		} else {
			result = podmanTest.Podman([]string{"run", "--rm", "--cpuset-mems=0", ALPINE, "cat", "/sys/fs/cgroup/cpuset/cpuset.mems"})
		}
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(Equal("0"))
	})

	It("podman run cpus and cpu-period", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-period=5000", "--cpus=0.5", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
	})

	It("podman run cpus and cpu-quota", func() {
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-quota=5000", "--cpus=0.5", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
	})

	It("podman run invalid cpu-rt-period with cgroupsv2", func() {
		SkipIfCgroupV1("testing options that only work in cgroup v2")
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-rt-period=5000", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.ErrorToString()).To(ContainSubstring("Realtime period not supported on cgroups V2 systems"))
	})

	It("podman run invalid cpu-rt-runtime with cgroupsv2", func() {
		SkipIfCgroupV1("testing options that only work in cgroup v2")
		result := podmanTest.Podman([]string{"run", "--rm", "--cpu-rt-runtime=5000", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.ErrorToString()).To(ContainSubstring("Realtime runtime not supported on cgroups V2 systems"))
	})
})
