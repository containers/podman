package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman pod pause", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	pausedState := "Paused"

	BeforeEach(func() {
		SkipIfRootlessCgroupsV1("Pause is not supported in cgroups v1")
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

	It("podman pod pause bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "pause", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman unpause bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "unpause", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman pod pause a created pod by id", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "pause", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("pause a running pod by id", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"pod", "pause", podid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"pod", "unpause", podid})
		result.WaitWithDefaultTimeout()
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("unpause a paused pod by id", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"pod", "pause", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).Should(HaveLen(1))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(Equal(pausedState))

		result = podmanTest.Podman([]string{"pod", "unpause", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).Should(HaveLen(1))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("unpause a paused pod by name", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"test1"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"pod", "pause", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).Should(HaveLen(1))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(Equal(pausedState))

		result = podmanTest.Podman([]string{"pod", "unpause", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).Should(HaveLen(1))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("unpause --all", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))
		_, ec, podid2 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.RunTopContainerInPod("", podid2)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"pod", "pause", podid1})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).Should(HaveLen(1))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"pod", "unpause", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).Should(HaveLen(1))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})
})
