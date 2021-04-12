package integration

import (
	"os"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman unshare", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)
	BeforeEach(func() {
		SkipIfRemote("podman-remote unshare is not supported")
		if _, err := os.Stat("/proc/self/uid_map"); err != nil {
			Skip("User namespaces not supported.")
		}

		if os.Geteuid() == 0 {
			Skip("Use unshare in rootless only")
		}

		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.CgroupManager = "cgroupfs"
		podmanTest.StorageOptions = ROOTLESS_STORAGE_OPTIONS
		podmanTest.Setup()
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
	})

	It("podman unshare", func() {
		userNS, _ := os.Readlink("/proc/self/ns/user")
		session := podmanTest.Podman([]string{"unshare", "readlink", "/proc/self/ns/user"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString(userNS)
		Expect(ok).To(BeFalse())
	})

	It("podman unshare --rootles-cni", func() {
		session := podmanTest.Podman([]string{"unshare", "--rootless-cni", "ip", "addr"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("tap0"))
	})
})
