package integration

import (
	"fmt"
	"os"
	"strconv"
	"time"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

// TODO: we need to check the output. Currently, we only check the exit codes
// which is not enough.
var _ = Describe("Podman stats", func() {
	var (
		tempdir    string
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootlessCgroupsV1("stats not supported on cgroupv1 for rootless users")
		if isContainerized() {
			SkipIfCgroupV1("stats not supported inside cgroupv1 container environment")
		}
		var err error
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman stats with bogus container", func() {
		session := podmanTest.Podman([]string{"stats", "--no-stream", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman stats on a running container", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"stats", "--no-stream", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman stats on all containers", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"stats", "--no-stream", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman stats on all running containers", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"stats", "--no-stream"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman stats only output cids", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"stats", "--all", "--no-trunc", "--no-stream", "--format", "\"{{.ID}}\""})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray()[0])).Should(BeEquivalentTo(66))
	})

	It("podman stats with GO template", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		stats := podmanTest.Podman([]string{"stats", "-a", "--no-reset", "--no-stream", "--format", "table {{.ID}} {{.AVGCPU}} {{.MemUsage}} {{.CPU}} {{.NetIO}} {{.BlockIO}} {{.PIDS}}"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).To(Exit(0))
	})

	It("podman stats with invalid GO template", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		stats := podmanTest.Podman([]string{"stats", "-a", "--no-reset", "--no-stream", "--format", "\"table {{.ID}} {{.NoSuchField}} \""})
		stats.WaitWithDefaultTimeout()
		Expect(stats).To(ExitWithError())
	})

	It("podman stats with negative interval", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		stats := podmanTest.Podman([]string{"stats", "-a", "--no-reset", "--no-stream", "--interval=-1"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).To(ExitWithError())
	})

	It("podman stats with zero interval", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		stats := podmanTest.Podman([]string{"stats", "-a", "--no-reset", "--no-stream", "--interval=0"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).To(ExitWithError())
	})

	It("podman stats with interval", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		stats := podmanTest.Podman([]string{"stats", "-a", "--no-reset", "--no-stream", "--interval=5"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))
	})

	It("podman stats with json output", func() {
		var found bool
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		for i := 0; i < 5; i++ {
			ps := podmanTest.Podman([]string{"ps", "-q"})
			ps.WaitWithDefaultTimeout()
			if len(ps.OutputToStringArray()) == 1 {
				found = true
				break
			}
			time.Sleep(time.Second)
		}
		Expect(found).To(BeTrue())
		stats := podmanTest.Podman([]string{"stats", "--all", "--no-stream", "--format", "json"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))
		Expect(stats.OutputToString()).To(BeValidJSON())
	})

	It("podman stats on a container with no net ns", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--net", "none", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"stats", "--no-stream", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman stats on a container that joined another's net ns", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "-d", "--net", fmt.Sprintf("container:%s", cid), ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"stats", "--no-stream", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman stats on container with forced slirp4netns", func() {
		// This will force the slirp4netns net mode to be tested as root
		session := podmanTest.Podman([]string{"run", "-d", "--net", "slirp4netns", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"stats", "--no-stream", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman reads slirp4netns network stats", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--network", "slirp4netns", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		cid := session.OutputToString()

		stats := podmanTest.Podman([]string{"stats", "--format", "'{{.NetIO}}'", "--no-stream", cid})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))
		Expect(stats.OutputToString()).To(Not(ContainSubstring("-- / --")))
	})

	// Regression test for #8265
	It("podman stats with custom memory limits", func() {
		// Run three containers. One with a memory limit.  Make sure
		// that the limits are different and the limited one has a
		// lower limit.
		ctrNoLimit0 := "no-limit-0"
		ctrNoLimit1 := "no-limit-1"
		ctrWithLimit := "with-limit"

		session := podmanTest.Podman([]string{"run", "-d", "--name", ctrNoLimit0, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "-d", "--name", ctrNoLimit1, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "-d", "--name", ctrWithLimit, "--memory", "50m", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"stats", "--no-stream", "--format", "{{.MemLimit}}", ctrNoLimit0, ctrNoLimit1, ctrWithLimit})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// We have three containers.  The unlimited ones need to have
		// the same limit, the limited one a lower one.
		limits := session.OutputToStringArray()
		Expect(limits).To(HaveLen(3))
		Expect(limits[0]).To(Equal(limits[1]))
		Expect(limits[0]).ToNot(Equal(limits[2]))

		defaultLimit, err := strconv.Atoi(limits[0])
		Expect(err).ToNot(HaveOccurred())
		customLimit, err := strconv.Atoi(limits[2])
		Expect(err).ToNot(HaveOccurred())

		Expect(customLimit).To(BeNumerically("<", defaultLimit))
	})

	It("podman stats with a container that is not running", func() {
		ctr := "created_container"
		session := podmanTest.Podman([]string{"create", "--name", ctr, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"stats", "--no-stream", ctr})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
})
