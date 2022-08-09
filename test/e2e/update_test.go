package integration

import (
	"github.com/containers/common/pkg/cgroupv2"
	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman update", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		Expect(err).To(BeNil())
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman update container all options v1", func() {
		SkipIfCgroupV2("testing flags that only work in cgroup v1")
		SkipIfRootless("many of these handlers are not enabled while rootless in CI")
		session := podmanTest.Podman([]string{"run", "-dt", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

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
			"--memory-swappiness", "50", ctrID}

		session = podmanTest.Podman(commonArgs)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// checking cpu quota from --cpus
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(ContainSubstring("500000"))

		// checking cpuset-cpus
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/cpuset/cpuset.cpus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(Equal("0"))

		// checking cpuset-mems
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/cpuset/cpuset.mems"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(Equal("0"))

		// checking memory limit
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/memory/memory.limit_in_bytes"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(ContainSubstring("1073741824"))

		// checking memory-swap
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(ContainSubstring("2147483648"))

		// checking cpu-shares
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/cpu/cpu.shares"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(ContainSubstring("123"))

	})

	It("podman update container all options v2", func() {
		SkipIfCgroupV1("testing flags that only work in cgroup v2")
		SkipIfRootless("many of these handlers are not enabled while rootless in CI")
		session := podmanTest.Podman([]string{"run", "-dt", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

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
			ctrID}

		session = podmanTest.Podman(commonArgs)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		ctrID = session.OutputToString()

		// checking cpu quota and period
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/cpu.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(ContainSubstring("500000"))

		// checking blkio weight
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/io.bfq.weight"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(ContainSubstring("123"))

		// checking device-read/write-bps/iops
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/io.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(ContainSubstring("rbps=10485760 wbps=10485760 riops=1000 wiops=1000"))

		// checking cpuset-cpus
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/cpuset.cpus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(Equal("0"))

		// checking cpuset-mems
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/cpuset.mems"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(Equal("0"))

		// checking memory limit
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/memory.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(ContainSubstring("1073741824"))

		// checking memory-swap
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/memory.swap.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(ContainSubstring("1073741824"))

		// checking cpu-shares
		session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/cpu.weight"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(ContainSubstring("5"))
	})

	It("podman update keep original resources if not overridden", func() {
		SkipIfRootless("many of these handlers are not enabled while rootless in CI")
		session := podmanTest.Podman([]string{"run", "-dt", "--cpus", "5", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{
			"update",
			"--memory", "1G",
			session.OutputToString(),
		})

		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		ctrID := session.OutputToString()

		if v2, _ := cgroupv2.Enabled(); v2 {
			session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/cpu.max"})
		} else {
			session = podmanTest.Podman([]string{"exec", "-it", ctrID, "cat", "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"})
		}
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(ContainSubstring("500000"))
	})
})
