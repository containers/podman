package integration

import (
	"fmt"
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run memory", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootlessCgroupsV1("Setting Memory not supported on cgroupv1 for rootless users")

		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman run memory test", func() {
		var session *PodmanSessionIntegration

		if CGROUPSV2 {
			session = podmanTest.Podman([]string{"run", "--memory=40m", "--net=none", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/memory.max"})
		} else {
			session = podmanTest.Podman([]string{"run", "--memory=40m", ALPINE, "cat", "/sys/fs/cgroup/memory/memory.limit_in_bytes"})
		}
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("41943040"))
	})

	It("podman run memory-reservation test", func() {
		if podmanTest.Host.Distribution == "ubuntu" {
			Skip("Unable to perform test on Ubuntu distributions due to memory management")
		}

		var session *PodmanSessionIntegration

		if CGROUPSV2 {
			session = podmanTest.Podman([]string{"run", "--memory-reservation=40m", "--net=none", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/memory.low"})
		} else {
			session = podmanTest.Podman([]string{"run", "--memory-reservation=40m", ALPINE, "cat", "/sys/fs/cgroup/memory/memory.soft_limit_in_bytes"})
		}

		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
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
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(expect))
	})

	for _, limit := range []string{"0", "15", "100"} {
		limit := limit // Keep this value in a proper scope
		testName := fmt.Sprintf("podman run memory-swappiness test(%s)", limit)
		It(testName, func() {
			SkipIfCgroupV2("memory-swappiness not supported on cgroupV2")
			session := podmanTest.Podman([]string{"run", fmt.Sprintf("--memory-swappiness=%s", limit), ALPINE, "cat", "/sys/fs/cgroup/memory/memory.swappiness"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.OutputToString()).To(Equal(limit))
		})
	}
})
