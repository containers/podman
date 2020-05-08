// +build !remoteclient

package integration

import (
	"fmt"
	"os"
	"time"

	"github.com/containers/libpod/pkg/cgroups"
	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// TODO: we need to check the output. Currently, we only check the exit codes
// which is not enough.
var _ = Describe("Podman stats", func() {
	var (
		tempdir    string
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		cgroupsv2, err := cgroups.IsCgroup2UnifiedMode()
		Expect(err).To(BeNil())

		if os.Geteuid() != 0 && !cgroupsv2 {
			Skip("This function is not enabled for rootless podman not running on cgroups v2")
		}
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
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

	It("podman stats with bogus container", func() {
		session := podmanTest.Podman([]string{"stats", "--no-stream", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman stats on a running container", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"stats", "--no-stream", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman stats on all containers", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"stats", "--no-stream", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman stats on all running containers", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"stats", "--no-stream"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman stats only output cids", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"stats", "--all", "--no-stream", "--format", "\"{{.ID}}\""})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman stats with json output", func() {
		var found bool
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
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
		Expect(stats.ExitCode()).To(Equal(0))
		Expect(stats.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman stats on a container with no net ns", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--net", "none", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"stats", "--no-stream", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman stats on a container that joined another's net ns", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "-d", "--net", fmt.Sprintf("container:%s", cid), ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"stats", "--no-stream", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
})
