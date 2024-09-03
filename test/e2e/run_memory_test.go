//go:build linux || freebsd

package integration

import (
	"fmt"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run memory", func() {

	BeforeEach(func() {
		SkipIfRootlessCgroupsV1("Setting Memory not supported on cgroupv1 for rootless users")
	})

	It("podman run memory test", func() {
		var session *PodmanSessionIntegration

		if CGROUPSV2 {
			session = podmanTest.Podman([]string{"run", "--memory=40m", "--net=none", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/memory.max"})
		} else {
			session = podmanTest.Podman([]string{"run", "--memory=40m", ALPINE, "cat", "/sys/fs/cgroup/memory/memory.limit_in_bytes"})
		}
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("41943040"))
	})

	It("podman run memory-reservation test", func() {
		var session *PodmanSessionIntegration

		if CGROUPSV2 {
			session = podmanTest.Podman([]string{"run", "--memory-reservation=40m", "--net=none", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/memory.low"})
		} else {
			session = podmanTest.Podman([]string{"run", "--memory-reservation=40m", ALPINE, "cat", "/sys/fs/cgroup/memory/memory.soft_limit_in_bytes"})
		}

		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("41943040"))
	})

	It("podman run memory-swap test", func() {
		var (
			session *PodmanSessionIntegration
			expect  string
		)

		if CGROUPSV2 {
			session = podmanTest.Podman([]string{"run", "--memory=20m", "--memory-swap=30M", "--net=none", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/memory.swap.max"})
			expect = "10485760"
		} else {
			session = podmanTest.Podman([]string{"run", "--memory=20m", "--memory-swap=30M", ALPINE, "cat", "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes"})
			expect = "31457280"
		}
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(expect))
	})

	for _, limit := range []string{"0", "15", "100"} {
		testName := fmt.Sprintf("podman run memory-swappiness test(%s)", limit)
		It(testName, func() {
			SkipIfCgroupV2("memory-swappiness not supported on cgroupV2")
			session := podmanTest.Podman([]string{"run", fmt.Sprintf("--memory-swappiness=%s", limit), ALPINE, "cat", "/sys/fs/cgroup/memory/memory.swappiness"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal(limit))
		})
	}

	It("podman run memory test on oomkilled container", func() {
		mem := SystemExec("cat", []string{"/proc/sys/vm/overcommit_memory"})
		mem.WaitWithDefaultTimeout()
		if mem.OutputToString() != "0" {
			Skip("overcommit memory is not set to 0")
		}

		ctrName := "oomkilled-ctr"
		// create a container that gets oomkilled
		session := podmanTest.Podman([]string{"run", "--name", ctrName, "--read-only", "--memory-swap=20m", "--memory=20m", "--oom-score-adj=1000", ALPINE, "sort", "/dev/urandom"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(137, ""))

		inspect := podmanTest.Podman(([]string{"inspect", "--format", "{{.State.OOMKilled}} {{.State.ExitCode}}", ctrName}))
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		// Check oomkilled and exit code values
		Expect(inspect.OutputToString()).Should(Equal("true 137"))
	})

	It("podman run memory test on successfully exited container", func() {
		ctrName := "success-ctr"
		session := podmanTest.Podman([]string{"run", "--name", ctrName, "--memory=40m", ALPINE, "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman(([]string{"inspect", "--format", "{{.State.OOMKilled}} {{.State.ExitCode}}", ctrName}))
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		// Check oomkilled and exit code values
		Expect(inspect.OutputToString()).Should(Equal("false 0"))
	})
})
