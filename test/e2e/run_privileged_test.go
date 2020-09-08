package integration

import (
	"os"
	"strconv"
	"strings"

	. "github.com/containers/podman/v2/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/syndtr/gocapability/capability"
)

// helper function for confirming that container capabilities are equal
// to those of the host, but only to the extent of caps we (podman)
// know about at compile time. That is: the kernel may have more caps
// available than we are aware of, leading to host=FFF... and ctr=3FF...
// because the latter is all we request. Accept that.
func containerCapMatchesHost(ctr_cap string, host_cap string) {
	ctr_cap_n, err := strconv.ParseUint(ctr_cap, 16, 64)
	Expect(err).NotTo(HaveOccurred(), "Error parsing %q as hex", ctr_cap)

	host_cap_n, err := strconv.ParseUint(host_cap, 16, 64)
	Expect(err).NotTo(HaveOccurred(), "Error parsing %q as hex", host_cap)

	// host caps can never be zero (except rootless, which we don't test).
	// and host caps must always be a superset (inclusive) of container
	Expect(host_cap_n).To(BeNumerically(">", 0), "host cap %q should be nonzero", host_cap)
	Expect(host_cap_n).To(BeNumerically(">=", ctr_cap_n), "host cap %q should never be less than container cap %q", host_cap, ctr_cap)

	host_cap_masked := host_cap_n & (1<<len(capability.List()) - 1)
	Expect(ctr_cap_n).To(Equal(host_cap_masked), "container cap %q is not a subset of host cap %q", ctr_cap, host_cap)
}

var _ = Describe("Podman privileged container tests", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
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

	It("podman privileged make sure sys is mounted rw", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "busybox", "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, lines := session.GrepString("sysfs")
		Expect(ok).To(BeTrue())
		Expect(lines[0]).To(ContainSubstring("sysfs (rw,"))
	})

	It("podman privileged CapEff", func() {
		SkipIfRootless()
		host_cap := SystemExec("awk", []string{"/^CapEff/ { print $2 }", "/proc/self/status"})
		Expect(host_cap.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "--privileged", "busybox", "awk", "/^CapEff/ { print $2 }", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		containerCapMatchesHost(session.OutputToString(), host_cap.OutputToString())
	})

	It("podman cap-add CapEff", func() {
		SkipIfRootless()
		// Get caps of current process
		host_cap := SystemExec("awk", []string{"/^CapEff/ { print $2 }", "/proc/self/status"})
		Expect(host_cap.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "--cap-add", "all", "busybox", "awk", "/^CapEff/ { print $2 }", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		containerCapMatchesHost(session.OutputToString(), host_cap.OutputToString())
	})

	It("podman cap-drop CapEff", func() {
		session := podmanTest.Podman([]string{"run", "--cap-drop", "all", "busybox", "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		capEff := strings.Split(session.OutputToString(), " ")
		Expect("0000000000000000").To(Equal(capEff[1]))
	})

	It("podman non-privileged should have very few devices", func() {
		session := podmanTest.Podman([]string{"run", "-t", "busybox", "ls", "-l", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(17))
	})

	It("podman privileged should inherit host devices", func() {
		SkipIfRootless()
		session := podmanTest.Podman([]string{"run", "--privileged", ALPINE, "ls", "-l", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 20))
	})

	It("run no-new-privileges test", func() {
		// Check if our kernel is new enough
		k, err := IsKernelNewerThan("4.14")
		Expect(err).To(BeNil())
		if !k {
			Skip("Kernel is not new enough to test this feature")
		}

		cap := SystemExec("grep", []string{"NoNewPrivs", "/proc/self/status"})
		if cap.ExitCode() != 0 {
			Skip("Can't determine NoNewPrivs")
		}

		session := podmanTest.Podman([]string{"run", "busybox", "grep", "NoNewPrivs", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		privs := strings.Split(session.OutputToString(), ":")
		session = podmanTest.Podman([]string{"run", "--security-opt", "no-new-privileges", "busybox", "grep", "NoNewPrivs", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		noprivs := strings.Split(session.OutputToString(), ":")
		Expect(privs[1]).To(Not(Equal(noprivs[1])))
	})

})
