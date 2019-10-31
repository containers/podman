// +build !remoteclient

package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/selinux/go-selinux"
)

var _ = Describe("Podman run", func() {
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
		if !selinux.GetEnabled() {
			Skip("SELinux not enabled")
		}
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman run selinux", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("container_t")
		Expect(match).Should(BeTrue())
	})

	It("podman run selinux grep test", func() {
		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", "label=level:s0:c1,c2", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("s0:c1,c2")
		Expect(match).Should(BeTrue())
	})

	It("podman run selinux disable test", func() {
		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", "label=disable", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("spc_t")
		Expect(match).Should(BeTrue())
	})

	It("podman run selinux type check test", func() {
		session := podmanTest.Podman([]string{"run", "-it", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match1, _ := session.GrepString("container_t")
		match2, _ := session.GrepString("svirt_lxc_net_t")
		Expect(match1 || match2).Should(BeTrue())
	})

	It("podman run selinux type setup test", func() {
		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", "label=type:spc_t", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("spc_t")
		Expect(match).Should(BeTrue())
	})

	It("podman privileged selinux", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("spc_t")
		Expect(match).Should(BeTrue())
	})

	It("podman test selinux label resolv.conf", func() {
		session := podmanTest.Podman([]string{"run", fedoraMinimal, "ls", "-Z", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("container_file_t")
		Expect(match).Should(BeTrue())
	})

	It("podman test selinux label hosts", func() {
		session := podmanTest.Podman([]string{"run", fedoraMinimal, "ls", "-Z", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("container_file_t")
		Expect(match).Should(BeTrue())
	})

	It("podman test selinux label hostname", func() {
		session := podmanTest.Podman([]string{"run", fedoraMinimal, "ls", "-Z", "/etc/hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("container_file_t")
		Expect(match).Should(BeTrue())
	})

	It("podman test selinux label /run/secrets", func() {
		SkipIfRootless()
		session := podmanTest.Podman([]string{"run", fedoraMinimal, "ls", "-dZ", "/run/secrets"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("container_file_t")
		Expect(match).Should(BeTrue())
	})

	It("podman test selinux --privileged label resolv.conf", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", fedoraMinimal, "ls", "-Z", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("container_file_t")
		Expect(match).Should(BeTrue())
	})

	It("podman test selinux --privileged label hosts", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", fedoraMinimal, "ls", "-Z", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("container_file_t")
		Expect(match).Should(BeTrue())
	})

	It("podman test selinux --privileged label hostname", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", fedoraMinimal, "ls", "-Z", "/etc/hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("container_file_t")
		Expect(match).Should(BeTrue())
	})

	It("podman test selinux --privileged label /run/secrets", func() {
		SkipIfRootless()
		session := podmanTest.Podman([]string{"run", "--privileged", fedoraMinimal, "ls", "-dZ", "/run/secrets"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("container_file_t")
		Expect(match).Should(BeTrue())
	})

	It("podman run selinux file type setup test", func() {
		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", "label=type:spc_t", "--security-opt", "label=filetype:container_var_lib_t", fedoraMinimal, "ls", "-Z", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		match, _ := session.GrepString("container_var_lib_t")
		Expect(match).Should(BeTrue())

		session = podmanTest.Podman([]string{"run", "-it", "--security-opt", "label=type:spc_t", "--security-opt", "label=filetype:foobar", fedoraMinimal, "ls", "-Z", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(126))
	})

	It("podman exec selinux check", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"exec", "test1", "cat", "/proc/1/attr/current"})
		session.WaitWithDefaultTimeout()
		session1 := podmanTest.Podman([]string{"exec", "test1", "cat", "/proc/self/attr/current"})
		session1.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Equal(session1.OutputToString()))
	})

})
