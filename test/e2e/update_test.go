package integration

import (
	"github.com/containers/common/pkg/cgroupv2"
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman update", func() {

	It("podman update container all options v1", func() {
		SkipIfCgroupV2("testing flags that only work in cgroup v1")
		SkipIfRootless("many of these handlers are not enabled while rootless in CI")
		session := podmanTest.Podman([]string{"run", "-dt", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		ctrID := session.OutputToString()

		commonArgs := []string{
			"update",
			"--cpus", "5",
			"--cpuset-cpus", "0",
			"--cpu-shares", "123",
			"--cpuset-mems", "0",
			"--memory", "1G",
			"--memory-swap", "2G",
			"--memory-reservation", "2G",
			"--memory-swappiness", "50",
			"--pids-limit", "123", ctrID}

		session = podmanTest.Podman(commonArgs)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// checking cpu quota from --cpus
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("500000"))

		// checking cpuset-cpus
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/cpuset/cpuset.cpus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("0"))

		// checking cpuset-mems
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/cpuset/cpuset.mems"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("0"))

		// checking memory limit
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/memory/memory.limit_in_bytes"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("1073741824"))

		// checking memory-swap
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("2147483648"))

		// checking cpu-shares
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/cpu/cpu.shares"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("123"))

		// checking pids-limit
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/pids/pids.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("123"))

	})

	It("podman update container unspecified pid limit", func() {
		SkipIfCgroupV1("testing flags that only work in cgroup v2")
		SkipIfRootless("many of these handlers are not enabled while rootless in CI")
		session := podmanTest.Podman([]string{"run", "-dt", "--pids-limit", "-1", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		ctrID := session.OutputToString()

		commonArgs := []string{
			"update",
			"--cpus", "5",
			ctrID}

		session = podmanTest.Podman(commonArgs)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		ctrID = session.OutputToString()

		// checking pids-limit was not changed after update when not specified as an option
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/pids.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("max"))
	})

	It("podman update container all options v2", func() {
		SkipIfCgroupV1("testing flags that only work in cgroup v2")
		SkipIfRootless("many of these handlers are not enabled while rootless in CI")
		session := podmanTest.Podman([]string{"run", "-dt", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		ctrID := session.OutputToString()

		commonArgs := []string{
			"update",
			"--cpus", "5",
			"--cpuset-cpus", "0",
			"--cpu-shares", "123",
			"--cpuset-mems", "0",
			"--memory", "1G",
			"--memory-swap", "2G",
			"--memory-reservation", "2G",
			"--blkio-weight", "123",
			"--device-read-bps", "/dev/zero:10mb",
			"--device-write-bps", "/dev/zero:10mb",
			"--device-read-iops", "/dev/zero:1000",
			"--device-write-iops", "/dev/zero:1000",
			"--pids-limit", "123",
			ctrID}

		session = podmanTest.Podman(commonArgs)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		ctrID = session.OutputToString()

		// checking cpu quota and period
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/cpu.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("500000"))

		// checking blkio weight
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/io.bfq.weight"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("123"))

		// checking device-read/write-bps/iops
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/io.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("rbps=10485760 wbps=10485760 riops=1000 wiops=1000"))

		// checking cpuset-cpus
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/cpuset.cpus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("0"))

		// checking cpuset-mems
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/cpuset.mems"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("0"))

		// checking memory limit
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/memory.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("1073741824"))

		// checking memory-swap
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/memory.swap.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("1073741824"))

		// checking cpu-shares
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/cpu.weight"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("5"))

		// checking pids-limit
		session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/pids.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("123"))
	})

	It("podman update keep original resources if not overridden", func() {
		SkipIfRootless("many of these handlers are not enabled while rootless in CI")
		session := podmanTest.Podman([]string{"run", "-dt", "--cpus", "5", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{
			"update",
			"--memory", "1G",
			session.OutputToString(),
		})

		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		ctrID := session.OutputToString()

		if v2, _ := cgroupv2.Enabled(); v2 {
			session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/cpu.max"})
		} else {
			session = podmanTest.Podman([]string{"exec", ctrID, "cat", "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"})
		}
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("500000"))
	})

	It("podman update persists changes", func() {
		SkipIfCgroupV1("testing flags that only work in cgroup v2")
		SkipIfRootless("many of these handlers are not enabled while rootless in CI")

		testCtr := "test-ctr-name"
		ctr1 := podmanTest.Podman([]string{"run", "-d", "--name", testCtr, "-m", "512m", ALPINE, "top"})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())

		inspect1 := podmanTest.Podman([]string{"inspect", "--format", "{{ .HostConfig.Memory }}", testCtr})
		inspect1.WaitWithDefaultTimeout()
		Expect(inspect1).Should(ExitCleanly())
		Expect(inspect1.OutputToString()).To(Equal("536870912"))

		exec1 := podmanTest.Podman([]string{"exec", testCtr, "cat", "/sys/fs/cgroup/memory.max"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(ExitCleanly())
		Expect(exec1.OutputToString()).Should(ContainSubstring("536870912"))

		update := podmanTest.Podman([]string{"update", "-m", "256m", testCtr})
		update.WaitWithDefaultTimeout()
		Expect(update).Should(ExitCleanly())

		inspect2 := podmanTest.Podman([]string{"inspect", "--format", "{{ .HostConfig.Memory }}", testCtr})
		inspect2.WaitWithDefaultTimeout()
		Expect(inspect2).Should(ExitCleanly())
		Expect(inspect2.OutputToString()).To(Equal("268435456"))

		exec2 := podmanTest.Podman([]string{"exec", testCtr, "cat", "/sys/fs/cgroup/memory.max"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())
		Expect(exec2.OutputToString()).Should(ContainSubstring("268435456"))

		restart := podmanTest.Podman([]string{"restart", testCtr})
		restart.WaitWithDefaultTimeout()
		Expect(restart).Should(ExitCleanly())

		inspect3 := podmanTest.Podman([]string{"inspect", "--format", "{{ .HostConfig.Memory }}", testCtr})
		inspect3.WaitWithDefaultTimeout()
		Expect(inspect3).Should(ExitCleanly())
		Expect(inspect3.OutputToString()).To(Equal("268435456"))

		exec3 := podmanTest.Podman([]string{"exec", testCtr, "cat", "/sys/fs/cgroup/memory.max"})
		exec3.WaitWithDefaultTimeout()
		Expect(exec3).Should(ExitCleanly())
		Expect(exec3.OutputToString()).Should(ContainSubstring("268435456"))

		pause := podmanTest.Podman([]string{"pause", testCtr})
		pause.WaitWithDefaultTimeout()
		Expect(pause).Should(ExitCleanly())

		update2 := podmanTest.Podman([]string{"update", "-m", "512m", testCtr})
		update2.WaitWithDefaultTimeout()
		Expect(update2).Should(ExitCleanly())

		unpause := podmanTest.Podman([]string{"unpause", testCtr})
		unpause.WaitWithDefaultTimeout()
		Expect(unpause).Should(ExitCleanly())

		inspect4 := podmanTest.Podman([]string{"inspect", "--format", "{{ .HostConfig.Memory }}", testCtr})
		inspect4.WaitWithDefaultTimeout()
		Expect(inspect4).Should(ExitCleanly())
		Expect(inspect4.OutputToString()).To(Equal("536870912"))

		exec4 := podmanTest.Podman([]string{"exec", testCtr, "cat", "/sys/fs/cgroup/memory.max"})
		exec4.WaitWithDefaultTimeout()
		Expect(exec4).Should(ExitCleanly())
		Expect(exec4.OutputToString()).Should(ContainSubstring("536870912"))
	})
})
