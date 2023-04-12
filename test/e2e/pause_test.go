package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		SkipIfRootlessCgroupsV1("Pause is not supported in cgroups v1")
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}

		if CGROUPSV2 {
			b, err := os.ReadFile("/proc/self/cgroup")
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentSpecReport()
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
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).To(ExitWithError())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(createdState))

		// check we can read stats for a paused container
		result = podmanTest.Podman([]string{"stats", "--no-stream", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman pause a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"unpause", cid})
		result.WaitWithDefaultTimeout()
	})

	It("podman container pause a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"container", "unpause", cid})
		result.WaitWithDefaultTimeout()
	})

	It("podman unpause a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"unpause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

	})

	It("podman remove a paused container by id without force", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(2))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

	})

	It("podman remove a paused container by id with force", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "--force", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman stop a paused container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"stop", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"unpause", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(2))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-f", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

	})

	It("podman pause a running container by name", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"pause", "test1"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(Equal(pausedState))

		result = podmanTest.Podman([]string{"unpause", "test1"})
		result.WaitWithDefaultTimeout()
	})

	It("podman pause a running container by id and another by name", func() {
		session1 := podmanTest.RunTopContainer("test1")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))

		session2 := podmanTest.RunTopContainer("")
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
		cid2 := session2.OutputToString()

		result := podmanTest.Podman([]string{"pause", cid2})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"pause", "test1"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		result = podmanTest.Podman([]string{"unpause", "test1"})
		result.WaitWithDefaultTimeout()
		result = podmanTest.Podman([]string{"unpause", cid2})
		result.WaitWithDefaultTimeout()
	})

	It("Pause all containers (no containers exist)", func() {
		result := podmanTest.Podman([]string{"pause", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

	})

	It("Unpause all containers (no paused containers exist)", func() {
		result := podmanTest.Podman([]string{"unpause", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("Pause a bunch of running containers", func() {
		for i := 0; i < 3; i++ {
			name := fmt.Sprintf("test%d", i)
			run := podmanTest.Podman([]string{"run", "-dt", "--name", name, NGINX_IMAGE})
			run.WaitWithDefaultTimeout()
			Expect(run).Should(Exit(0))

		}
		running := podmanTest.Podman([]string{"ps", "-q"})
		running.WaitWithDefaultTimeout()
		Expect(running).Should(Exit(0))
		Expect(running.OutputToStringArray()).To(HaveLen(3))

		pause := podmanTest.Podman([]string{"pause", "--all"})
		pause.WaitWithDefaultTimeout()
		Expect(pause).Should(Exit(0))

		running = podmanTest.Podman([]string{"ps", "-q"})
		running.WaitWithDefaultTimeout()
		Expect(running).Should(Exit(0))
		Expect(running.OutputToStringArray()).To(BeEmpty())

		unpause := podmanTest.Podman([]string{"unpause", "--all"})
		unpause.WaitWithDefaultTimeout()
		Expect(unpause).Should(Exit(0))
	})

	It("Unpause a bunch of running containers", func() {
		for i := 0; i < 3; i++ {
			name := fmt.Sprintf("test%d", i)
			run := podmanTest.Podman([]string{"run", "-dt", "--name", name, NGINX_IMAGE})
			run.WaitWithDefaultTimeout()
			Expect(run).Should(Exit(0))

		}
		pause := podmanTest.Podman([]string{"pause", "--all"})
		pause.WaitWithDefaultTimeout()
		Expect(pause).Should(Exit(0))

		unpause := podmanTest.Podman([]string{"unpause", "--all"})
		unpause.WaitWithDefaultTimeout()
		Expect(unpause).Should(Exit(0))

		running := podmanTest.Podman([]string{"ps", "-q"})
		running.WaitWithDefaultTimeout()
		Expect(running).Should(Exit(0))
		Expect(running.OutputToStringArray()).To(HaveLen(3))
	})

	It("podman pause --latest", func() {
		SkipIfRemote("--latest flag n/a")
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		result := podmanTest.Podman([]string{"pause", "-l"})
		result.WaitWithDefaultTimeout()

		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(strings.ToLower(podmanTest.GetContainerStatus())).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"unpause", "-l"})
		result.WaitWithDefaultTimeout()
	})

	It("podman pause --cidfile", func() {
		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		tmpFile := tmpDir + "cid"

		defer os.RemoveAll(tmpDir)

		session := podmanTest.Podman([]string{"create", "--cidfile", tmpFile, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToStringArray()[0]

		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"pause", "--cidfile", tmpFile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid))

		result = podmanTest.Podman([]string{"unpause", "--cidfile", tmpFile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		output = result.OutputToString()
		Expect(output).To(ContainSubstring(cid))
	})

	It("podman pause multiple --cidfile", func() {
		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		tmpFile1 := tmpDir + "cid-1"
		tmpFile2 := tmpDir + "cid-2"

		defer os.RemoveAll(tmpDir)

		session := podmanTest.Podman([]string{"run", "--cidfile", tmpFile1, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid1 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"run", "--cidfile", tmpFile2, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid2 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))

		result := podmanTest.Podman([]string{"pause", "--cidfile", tmpFile1, "--cidfile", tmpFile2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))

		result = podmanTest.Podman([]string{"unpause", "--cidfile", tmpFile1, "--cidfile", tmpFile2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		output = result.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))
	})

	It("podman pause invalid --latest and --cidfile and --all", func() {
		SkipIfRemote("--latest flag n/a")
		result := podmanTest.Podman([]string{"pause", "--cidfile", "foobar", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("cannot be used together"))
		result = podmanTest.Podman([]string{"pause", "--cidfile", "foobar", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("cannot be used together"))
		result = podmanTest.Podman([]string{"pause", "--cidfile", "foobar", "--all", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("cannot be used together"))
		result = podmanTest.Podman([]string{"pause", "--latest", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("cannot be used together"))
	})

	It("podman unpause invalid --latest and --cidfile and --all", func() {
		SkipIfRemote("--latest flag n/a")
		result := podmanTest.Podman([]string{"unpause", "--cidfile", "foobar", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("cannot be used together"))
		result = podmanTest.Podman([]string{"unpause", "--cidfile", "foobar", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("cannot be used together"))
		result = podmanTest.Podman([]string{"unpause", "--cidfile", "foobar", "--all", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("cannot be used together"))
		result = podmanTest.Podman([]string{"unpause", "--latest", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("cannot be used together"))
	})

	It("podman pause --filter", func() {
		session1 := podmanTest.RunTopContainer("")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		cid1 := session1.OutputToString()

		session1 = podmanTest.RunTopContainer("")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		cid2 := session1.OutputToString()

		session1 = podmanTest.RunTopContainer("")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		cid3 := session1.OutputToString()
		shortCid3 := cid3[0:5]

		session1 = podmanTest.Podman([]string{"pause", cid1, "-f", "status=test"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(125))

		session1 = podmanTest.Podman([]string{"unpause", cid1, "-f", "status=paused"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(125))

		session1 = podmanTest.Podman([]string{"pause", "-a", "--filter", fmt.Sprintf("id=%swrongid", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		Expect(session1.OutputToString()).To(HaveLen(0))

		session1 = podmanTest.Podman([]string{"pause", "-a", "--filter", fmt.Sprintf("id=%s", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"unpause", "-a", "--filter", fmt.Sprintf("id=%swrongid", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		Expect(session1.OutputToString()).To(HaveLen(0))

		session1 = podmanTest.Podman([]string{"unpause", "-a", "--filter", fmt.Sprintf("id=%s", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"pause", "-f", fmt.Sprintf("id=%s", cid2)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid2))

		session1 = podmanTest.Podman([]string{"unpause", "-f", fmt.Sprintf("id=%s", cid2)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid2))
	})
})
