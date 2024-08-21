//go:build linux || freebsd

package integration

import (
	"github.com/containers/common/pkg/cgroupv2"
	. "github.com/containers/podman/v5/test/utils"
	"github.com/containers/storage/pkg/fileutils"
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
		podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/cpu/cpu.cfs_quota_us", "500000")

		// checking cpuset-cpus
		podmanTest.CheckFileInContainer(ctrID, "/sys/fs/cgroup/cpuset/cpuset.cpus", "0")

		// checking cpuset-mems
		podmanTest.CheckFileInContainer(ctrID, "/sys/fs/cgroup/cpuset/cpuset.mems", "0")

		// checking memory limit
		podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/memory/memory.limit_in_bytes", "1073741824")

		// checking memory-swap
		podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes", "2147483648")

		// checking cpu-shares
		podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/cpu/cpu.shares", "123")

		// checking pids-limit
		podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/pids/pids.max", "123")
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
		podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/pids.max", "max")
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
		podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/cpu.max", "500000")

		// checking blkio weight (as of 2024-05 this file does not exist on Debian 13)
		if err := fileutils.Exists("/sys/fs/cgroup/system.slice/io.bfq.weight"); err == nil {
			podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/io.bfq.weight", "123")
		}

		// checking device-read/write-bps/iops
		podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/io.max", "rbps=10485760 wbps=10485760 riops=1000 wiops=1000")

		// checking cpuset-cpus
		podmanTest.CheckFileInContainer(ctrID, "/sys/fs/cgroup/cpuset.cpus", "0")

		// checking cpuset-mems
		podmanTest.CheckFileInContainer(ctrID, "/sys/fs/cgroup/cpuset.mems", "0")

		// checking memory limit
		podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/memory.max", "1073741824")

		// checking memory-swap
		podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/memory.swap.max", "1073741824")

		// checking cpu-shares
		podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/cpu.weight", "5")

		// checking pids-limit
		podmanTest.CheckFileInContainerSubstring(ctrID, "/sys/fs/cgroup/pids.max", "123")
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

		path := "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"
		if v2, _ := cgroupv2.Enabled(); v2 {
			path = "/sys/fs/cgroup/cpu.max"
		}

		podmanTest.CheckFileInContainerSubstring(ctrID, path, "500000")
	})

	It("podman update persists changes", func() {
		SkipIfCgroupV1("testing flags that only work in cgroup v2")
		SkipIfRootless("many of these handlers are not enabled while rootless in CI")

		memoryInspect := ".HostConfig.Memory"
		memoryCgroup := "/sys/fs/cgroup/memory.max"
		mem512m := "536870912"
		mem256m := "268435456"

		testCtr := "test-ctr-name"
		ctr1 := podmanTest.Podman([]string{"run", "-d", "--name", testCtr, "-m", "512m", ALPINE, "top"})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())

		podmanTest.CheckContainerSingleField(testCtr, memoryInspect, mem512m)
		podmanTest.CheckFileInContainer(testCtr, memoryCgroup, mem512m)

		update := podmanTest.Podman([]string{"update", "-m", "256m", testCtr})
		update.WaitWithDefaultTimeout()
		Expect(update).Should(ExitCleanly())

		podmanTest.CheckContainerSingleField(testCtr, memoryInspect, mem256m)
		podmanTest.CheckFileInContainer(testCtr, memoryCgroup, mem256m)

		restart := podmanTest.Podman([]string{"restart", testCtr})
		restart.WaitWithDefaultTimeout()
		Expect(restart).Should(ExitCleanly())

		podmanTest.CheckContainerSingleField(testCtr, memoryInspect, mem256m)
		podmanTest.CheckFileInContainer(testCtr, memoryCgroup, mem256m)

		pause := podmanTest.Podman([]string{"pause", testCtr})
		pause.WaitWithDefaultTimeout()
		Expect(pause).Should(ExitCleanly())

		update2 := podmanTest.Podman([]string{"update", "-m", "512m", testCtr})
		update2.WaitWithDefaultTimeout()
		Expect(update2).Should(ExitCleanly())

		unpause := podmanTest.Podman([]string{"unpause", testCtr})
		unpause.WaitWithDefaultTimeout()
		Expect(unpause).Should(ExitCleanly())

		podmanTest.CheckContainerSingleField(testCtr, memoryInspect, mem512m)
		podmanTest.CheckFileInContainer(testCtr, memoryCgroup, mem512m)
	})

	It("podman update sets restart policy", func() {
		restartPolicyName := ".HostConfig.RestartPolicy.Name"
		restartPolicyRetries := ".HostConfig.RestartPolicy.MaximumRetryCount"

		testCtr := "test-ctr-name"
		ctr1 := podmanTest.Podman([]string{"run", "-dt", "--name", testCtr, ALPINE, "top"})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())

		podmanTest.CheckContainerSingleField(testCtr, restartPolicyName, "no")
		podmanTest.CheckContainerSingleField(testCtr, restartPolicyRetries, "0")

		update1 := podmanTest.Podman([]string{"update", "--restart", "on-failure:5", testCtr})
		update1.WaitWithDefaultTimeout()
		Expect(update1).Should(ExitCleanly())

		podmanTest.CheckContainerSingleField(testCtr, restartPolicyName, "on-failure")
		podmanTest.CheckContainerSingleField(testCtr, restartPolicyRetries, "5")

		update2 := podmanTest.Podman([]string{"update", "--restart", "always", testCtr})
		update2.WaitWithDefaultTimeout()
		Expect(update2).Should(ExitCleanly())

		podmanTest.CheckContainerSingleField(testCtr, restartPolicyName, "always")
		podmanTest.CheckContainerSingleField(testCtr, restartPolicyRetries, "0")
	})
})
