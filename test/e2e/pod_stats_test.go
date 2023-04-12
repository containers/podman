package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman pod stats", func() {
	var (
		err        error
		tempdir    string
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootlessCgroupsV1("Tests fail with both CGv1 + required --cgroup-manager=cgroupfs")
		if isContainerized() {
			SkipIfCgroupV1("All tests fail Error: unable to load cgroup at ...: cgroup deleted")
		}

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
	It("podman pod stats should run with no pods", func() {
		session := podmanTest.Podman([]string{"pod", "stats", "--no-stream"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman pod stats with a bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "stats", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman pod stats on a specific running pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream", podid})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))
	})

	It("podman pod stats on a specific running pod with shortID", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream", podid[:5]})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))
	})

	It("podman pod stats on a specific running pod with name", func() {
		_, ec, podid := podmanTest.CreatePod(map[string][]string{"--name": {"test"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream", "test"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))
	})

	It("podman pod stats on running pods", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))
	})

	It("podman pod stats on all pods", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream", "-a"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))
	})

	It("podman pod stats with json output", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--format", "json", "--no-stream", "-a"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))
		Expect(stats.OutputToString()).To(BeValidJSON())
	})
	It("podman pod stats with GO template", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		stats := podmanTest.Podman([]string{"pod", "stats", "-a", "--no-reset", "--no-stream", "--format", "table {{.CID}} {{.Pod}} {{.Mem}} {{.MemUsage}} {{.CPU}} {{.NetIO}} {{.BlockIO}} {{.PIDS}} {{.Pod}}"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).To(Exit(0))
	})

	It("podman pod stats with invalid GO template", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		stats := podmanTest.Podman([]string{"pod", "stats", "-a", "--no-reset", "--no-stream", "--format", "\"table {{.ID}} \""})
		stats.WaitWithDefaultTimeout()
		Expect(stats).To(ExitWithError())
	})

	It("podman pod stats on net=host post", func() {
		SkipIfRootless("--net=host not supported for rootless pods at present")
		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--net=host", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		ctrRun := podmanTest.Podman([]string{"run", "-d", "--pod", podName, ALPINE, "top"})
		ctrRun.WaitWithDefaultTimeout()
		Expect(ctrRun).Should(Exit(0))

		stats := podmanTest.Podman([]string{"pod", "stats", "--format", "json", "--no-stream", podName})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))
		Expect(stats.OutputToString()).To(BeValidJSON())
	})
})
