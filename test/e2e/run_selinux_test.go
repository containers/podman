package integration

import (
	"os"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_t"))
	})

	It("podman run selinux grep test", func() {
		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", "label=level:s0:c1,c2", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("s0:c1,c2"))
	})

	It("podman run selinux disable test", func() {
		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", "label=disable", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("spc_t"))
	})

	It("podman run selinux type check test", func() {
		session := podmanTest.Podman([]string{"run", "-it", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		match1, _ := session.GrepString("container_t")
		match2, _ := session.GrepString("svirt_lxc_net_t")
		Expect(match1 || match2).Should(BeTrue())
	})

	It("podman run selinux type setup test", func() {
		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", "label=type:spc_t", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("spc_t"))
	})

	It("podman privileged selinux", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("spc_t"))
	})

	It("podman test selinux label resolv.conf", func() {
		session := podmanTest.Podman([]string{"run", fedoraMinimal, "ls", "-Z", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux label hosts", func() {
		session := podmanTest.Podman([]string{"run", fedoraMinimal, "ls", "-Z", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux label hostname", func() {
		session := podmanTest.Podman([]string{"run", fedoraMinimal, "ls", "-Z", "/etc/hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux label /run/secrets", func() {
		session := podmanTest.Podman([]string{"run", fedoraMinimal, "ls", "-dZ", "/run/secrets"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux --privileged label resolv.conf", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", fedoraMinimal, "ls", "-Z", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux --privileged label hosts", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", fedoraMinimal, "ls", "-Z", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux --privileged label hostname", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", fedoraMinimal, "ls", "-Z", "/etc/hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman test selinux --privileged label /run/secrets", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", fedoraMinimal, "ls", "-dZ", "/run/secrets"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_file_t"))
	})

	It("podman run selinux file type setup test", func() {
		session := podmanTest.Podman([]string{"run", "-it", "--security-opt", "label=type:spc_t", "--security-opt", "label=filetype:container_var_lib_t", fedoraMinimal, "ls", "-Z", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_var_lib_t"))

		session = podmanTest.Podman([]string{"run", "-it", "--security-opt", "label=type:spc_t", "--security-opt", "label=filetype:foobar", fedoraMinimal, "ls", "-Z", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(126))
	})

	It("podman exec selinux check", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "test1", "cat", "/proc/1/attr/current"})
		session.WaitWithDefaultTimeout()
		session1 := podmanTest.Podman([]string{"exec", "test1", "cat", "/proc/self/attr/current"})
		session1.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Equal(session1.OutputToString()))
	})

	It("podman run --privileged and --security-opt SELinux options", func() {
		session := podmanTest.Podman([]string{"run", "-it", "--privileged", "--security-opt", "label=type:spc_t", "--security-opt", "label=level:s0:c1,c2", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("spc_t"))
		Expect(session.OutputToString()).To(ContainSubstring("s0:c1,c2"))
	})

	It("podman pod container share SELinux labels", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--pod", podID, ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		label1 := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--pod", podID, ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(label1))

		session = podmanTest.Podman([]string{"pod", "rm", "-t", "0", podID, "--force"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman pod container --infra=false doesn't share SELinux labels", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--infra=false"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--pod", podID, ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		label1 := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--pod", podID, ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(Equal(label1)))

		session = podmanTest.Podman([]string{"pod", "rm", "-t", "0", podID, "--force"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman shared IPC NS container share SELinux labels", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "test1", "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		label1 := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--ipc", "container:test1", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(label1))
	})

	It("podman shared PID NS container share SELinux labels", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "test1", "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		label1 := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--pid", "container:test1", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(label1))
	})

	It("podman shared NET NS container doesn't share SELinux labels", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"exec", "test1", "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		label1 := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--net", "container:test1", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(Equal(label1)))
	})

	It("podman test --pid=host", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		session := podmanTest.Podman([]string{"run", "--pid=host", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("spc_t"))
	})

	It("podman test --ipc=host", func() {
		session := podmanTest.Podman([]string{"run", "--ipc=host", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("spc_t"))
	})

	It("podman test --ipc=net", func() {
		session := podmanTest.Podman([]string{"run", "--net=host", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_t"))
	})

	It("podman test --ipc=net", func() {
		session := podmanTest.Podman([]string{"run", "--net=host", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_t"))
	})

	It("podman test --ipc=net", func() {
		session := podmanTest.Podman([]string{"run", "--net=host", ALPINE, "cat", "/proc/self/attr/current"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("container_t"))
	})

	It("podman test --runtime=/PATHTO/kata-runtime", func() {
		runtime := podmanTest.OCIRuntime
		podmanTest.OCIRuntime = filepath.Join(podmanTest.TempDir, "kata-runtime")
		err := os.Symlink("/bin/true", podmanTest.OCIRuntime)
		Expect(err).To(BeNil())
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		}
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"inspect", "--format", "{{ .ProcessLabel }}", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("container_kvm_t"))

		podmanTest.OCIRuntime = runtime
		if IsRemote() {
			podmanTest.StopRemoteService()
			podmanTest.StartRemoteService()
		}
	})

	It("podman test init labels", func() {
		session := podmanTest.Podman([]string{"create", UBI_INIT, "/sbin/init"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"inspect", "--format", "{{ .ProcessLabel }}", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("container_init_t"))
	})

	It("podman relabels named volume with :Z", func() {
		session := podmanTest.Podman([]string{"run", "-v", "testvol:/test1/test:Z", fedoraMinimal, "ls", "-alZ", "/test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(":s0:"))
	})
})
