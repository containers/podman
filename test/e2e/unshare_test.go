package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).ToNot(ContainSubstring(userNS))
	})

	It("podman unshare --rootles-cni", func() {
		session := podmanTest.Podman([]string{"unshare", "--rootless-netns", "ip", "addr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("tap0"))
	})

	It("podman unshare exit codes", func() {
		session := podmanTest.Podman([]string{"unshare", "false"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
		Expect(session.OutputToString()).Should(Equal(""))
		Expect(session.ErrorToString()).Should(Equal(""))

		session = podmanTest.Podman([]string{"unshare", "/usr/bin/bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(127))
		Expect(session.OutputToString()).Should(Equal(""))
		Expect(session.ErrorToString()).Should(ContainSubstring("no such file or directory"))

		session = podmanTest.Podman([]string{"unshare", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(127))
		Expect(session.OutputToString()).Should(Equal(""))
		Expect(session.ErrorToString()).Should(ContainSubstring("executable file not found in $PATH"))

		session = podmanTest.Podman([]string{"unshare", "/usr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(126))
		Expect(session.OutputToString()).Should(Equal(""))
		Expect(session.ErrorToString()).Should(ContainSubstring("permission denied"))

		session = podmanTest.Podman([]string{"unshare", "--bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.OutputToString()).Should(Equal(""))
		Expect(session.ErrorToString()).Should(ContainSubstring("unknown flag: --bogus"))
	})
})
