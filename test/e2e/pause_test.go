package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/libpod/pkg/cgroups"
	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pause", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	pausedState := "paused"
	createdState := "created"

	BeforeEach(func() {
		SkipIfRootless()
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}

		cgroupsv2, err := cgroups.IsCgroup2UnifiedMode()
		Expect(err).To(BeNil())

		if cgroupsv2 {
			b, err := ioutil.ReadFile("/proc/self/cgroup")
			if err != nil {
				Skip("cannot read self cgroup")
			}

			path := filepath.Join("/sys/fs/cgroup", strings.TrimSuffix(strings.Replace(string(b), "0::", "", 1), "\n"), "cgroup.freeze")
			_, err = os.Stat(path)
			if err != nil {
				Skip("freezer controller not available on the current kernel")
			}
		}

		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman pause bogus container", func() {
		session := podmanTest.Podman([]string{"pause", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman unpause bogus container", func() {
		session := podmanTest.Podman([]string{"unpause", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman pause a created container by id", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).To(ExitWithError())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(createdState))
	})

	It("podman pause a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()
		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"unpause", cid})
		result.WaitWithDefaultTimeout()
	})

	It("podman container pause a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"container", "unpause", cid})
		result.WaitWithDefaultTimeout()
	})

	It("podman unpause a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"unpause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

	})

	It("podman remove a paused container by id without force", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(2))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

	})

	It("podman remove a paused container by id with force", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"rm", "--force", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman stop a paused container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"stop", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"unpause", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(2))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", "-f", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

	})

	It("podman pause a running container by name", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"pause", "test1"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(Equal(pausedState))

		result = podmanTest.Podman([]string{"unpause", "test1"})
		result.WaitWithDefaultTimeout()
	})

	It("podman pause a running container by id and another by name", func() {
		session1 := podmanTest.RunTopContainer("test1")
		session1.WaitWithDefaultTimeout()
		Expect(session1.ExitCode()).To(Equal(0))

		session2 := podmanTest.RunTopContainer("")
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))
		cid2 := session2.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid2})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"pause", "test1"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		result = podmanTest.Podman([]string{"unpause", "test1"})
		result.WaitWithDefaultTimeout()
		result = podmanTest.Podman([]string{"unpause", cid2})
		result.WaitWithDefaultTimeout()
	})

	It("Pause all containers (no containers exist)", func() {
		result := podmanTest.Podman([]string{"pause", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

	})

	It("Unpause all containers (no paused containers exist)", func() {
		result := podmanTest.Podman([]string{"unpause", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("Pause a bunch of running containers", func() {
		for i := 0; i < 3; i++ {
			name := fmt.Sprintf("test%d", i)
			run := podmanTest.Podman([]string{"run", "-dt", "--name", name, nginx})
			run.WaitWithDefaultTimeout()
			Expect(run.ExitCode()).To(Equal(0))

		}
		running := podmanTest.Podman([]string{"ps", "-q"})
		running.WaitWithDefaultTimeout()
		Expect(running.ExitCode()).To(Equal(0))
		Expect(len(running.OutputToStringArray())).To(Equal(3))

		pause := podmanTest.Podman([]string{"pause", "--all"})
		pause.WaitWithDefaultTimeout()
		Expect(pause.ExitCode()).To(Equal(0))

		running = podmanTest.Podman([]string{"ps", "-q"})
		running.WaitWithDefaultTimeout()
		Expect(running.ExitCode()).To(Equal(0))
		Expect(len(running.OutputToStringArray())).To(Equal(0))

		unpause := podmanTest.Podman([]string{"unpause", "--all"})
		unpause.WaitWithDefaultTimeout()
		Expect(unpause.ExitCode()).To(Equal(0))
	})

	It("Unpause a bunch of running containers", func() {
		for i := 0; i < 3; i++ {
			name := fmt.Sprintf("test%d", i)
			run := podmanTest.Podman([]string{"run", "-dt", "--name", name, nginx})
			run.WaitWithDefaultTimeout()
			Expect(run.ExitCode()).To(Equal(0))

		}
		pause := podmanTest.Podman([]string{"pause", "--all"})
		pause.WaitWithDefaultTimeout()
		Expect(pause.ExitCode()).To(Equal(0))

		unpause := podmanTest.Podman([]string{"unpause", "--all"})
		unpause.WaitWithDefaultTimeout()
		Expect(unpause.ExitCode()).To(Equal(0))

		running := podmanTest.Podman([]string{"ps", "-q"})
		running.WaitWithDefaultTimeout()
		Expect(running.ExitCode()).To(Equal(0))
		Expect(len(running.OutputToStringArray())).To(Equal(3))
	})

})
