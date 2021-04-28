package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		os.Setenv("CONTAINERS_CONF", "config/containers.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
		os.Unsetenv("CONTAINERS_CONF")
	})

	It("podman run limits test", func() {
		SkipIfRootlessCgroupsV1("Setting limits not supported on cgroupv1 for rootless users")
		//containers.conf is set to "nofile=500:500"
		session := podmanTest.Podman([]string{"run", "--rm", fedoraMinimal, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("500"))

		session = podmanTest.Podman([]string{"run", "--rm", "--ulimit", "nofile=2048:2048", fedoraMinimal, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("2048"))
	})

	It("podman run with containers.conf having additional env", func() {
		//containers.conf default env includes foo
		session := podmanTest.Podman([]string{"run", ALPINE, "printenv"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("foo=bar"))
	})

	It("podman run with additional devices", func() {
		//containers.conf devices includes notone
		session := podmanTest.Podman([]string{"run", "--device", "/dev/null:/dev/bar", ALPINE, "ls", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("bar"))
		Expect(session.OutputToString()).To(ContainSubstring("notone"))
	})

	It("podman run shm-size", func() {
		//containers.conf default sets shm-size=201k, which ends up as 200k
		session := podmanTest.Podman([]string{"run", ALPINE, "grep", "shm", "/proc/self/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("size=200k"))
	})

	It("podman Capabilities in containers.conf", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CGroupsV1")
		cap := podmanTest.Podman([]string{"run", ALPINE, "grep", "CapEff", "/proc/self/status"})
		cap.WaitWithDefaultTimeout()
		Expect(cap.ExitCode()).To(Equal(0))

		os.Setenv("CONTAINERS_CONF", "config/containers-ns.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		session := podmanTest.Podman([]string{"run", BB, "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).ToNot(Equal(cap.OutputToString()))
	})

	It("podman Regular capabilities", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		result := podmanTest.Podman([]string{"top", "test1", "capeff"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(ContainSubstring("SYS_CHROOT"))
		Expect(result.OutputToString()).To(ContainSubstring("NET_RAW"))
	})

	It("podman drop capabilities", func() {
		os.Setenv("CONTAINERS_CONF", "config/containers-caps.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		result := podmanTest.Podman([]string{"container", "top", "test1", "capeff"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).ToNot(ContainSubstring("SYS_CHROOT"))
		Expect(result.OutputToString()).ToNot(ContainSubstring("NET_RAW"))
	})

	verifyNSHandling := func(nspath, option string) {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CGroupsV1")
		os.Setenv("CONTAINERS_CONF", "config/containers-ns.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		//containers.conf default ipcns to default to host
		session := podmanTest.Podman([]string{"run", ALPINE, "ls", "-l", nspath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		fields := strings.Split(session.OutputToString(), " ")
		ctrNS := strings.TrimSuffix(fields[len(fields)-1], "\n")

		cmd := exec.Command("ls", "-l", nspath)
		res, err := cmd.Output()
		Expect(err).To(BeNil())
		fields = strings.Split(string(res), " ")
		hostNS := strings.TrimSuffix(fields[len(fields)-1], "\n")
		Expect(hostNS).To(Equal(ctrNS))

		session = podmanTest.Podman([]string{"run", option, "private", ALPINE, "ls", "-l", nspath})
		fields = strings.Split(session.OutputToString(), " ")
		ctrNS = fields[len(fields)-1]
		Expect(hostNS).ToNot(Equal(ctrNS))
	}

	It("podman compare netns", func() {
		verifyNSHandling("/proc/self/ns/net", "--network")
	})

	It("podman compare ipcns", func() {
		verifyNSHandling("/proc/self/ns/ipc", "--ipc")
	})

	It("podman compare utsns", func() {
		verifyNSHandling("/proc/self/ns/uts", "--uts")
	})

	It("podman compare pidns", func() {
		verifyNSHandling("/proc/self/ns/pid", "--pid")
	})

	It("podman compare cgroupns", func() {
		verifyNSHandling("/proc/self/ns/cgroup", "--cgroupns")
	})

	It("podman containers.conf additionalvolumes", func() {
		conffile := filepath.Join(podmanTest.TempDir, "container.conf")
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		err := ioutil.WriteFile(conffile, []byte(fmt.Sprintf("[containers]\nvolumes=[\"%s:%s:Z\",]\n", tempdir, tempdir)), 0755)
		if err != nil {
			os.Exit(1)
		}

		os.Setenv("CONTAINERS_CONF", conffile)
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		result := podmanTest.Podman([]string{"run", ALPINE, "ls", tempdir})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman run containers.conf sysctl test", func() {
		//containers.conf is set to   "net.ipv4.ping_group_range=0 1000"
		session := podmanTest.Podman([]string{"run", "--rm", fedoraMinimal, "cat", "/proc/sys/net/ipv4/ping_group_range"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("1000"))

		// Ignore containers.conf setting if --net=host
		session = podmanTest.Podman([]string{"run", "--rm", "--net", "host", fedoraMinimal, "cat", "/proc/sys/net/ipv4/ping_group_range"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).ToNot((ContainSubstring("1000")))
	})

	It("podman run containers.conf search domain", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session.LineInOutputStartsWith("search foobar.com")
	})

	It("podman run add dns server", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session.LineInOutputStartsWith("server 1.2.3.4")
	})

	It("podman run add dns option", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session.LineInOutputStartsWith("options debug")
	})

	It("podman run containers.conf remove all search domain", func() {
		session := podmanTest.Podman([]string{"run", "--dns-search=.", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputStartsWith("search")).To(BeFalse())
	})

	It("podman run use containers.conf search domain", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputStartsWith("search")).To(BeTrue())
		Expect(session.OutputToString()).To(ContainSubstring("foobar.com"))

		Expect(session.OutputToString()).To(ContainSubstring("1.2.3.4"))
		Expect(session.OutputToString()).To(ContainSubstring("debug"))
	})

	It("podman run containers.conf timezone", func() {
		//containers.conf timezone set to Pacific/Honolulu
		session := podmanTest.Podman([]string{"run", ALPINE, "date", "+'%H %Z'"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("HST"))
	})

	It("podman run containers.conf umask", func() {
		//containers.conf umask set to 0002
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test only works on crun")
		}

		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "sh", "-c", "umask"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("0002"))
	})

	It("podman set network cmd options slirp options to allow host loopback", func() {
		session := podmanTest.Podman([]string{"run", "--network", "slirp4netns", ALPINE, "ping", "-c1", "10.0.2.2"})
		session.Wait(30)
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman-remote test localcontainers.conf versus remote containers.conf", func() {
		if !IsRemote() {
			Skip("this test is only for remote")
		}

		os.Setenv("CONTAINERS_CONF", "config/containers-remote.conf")
		// Configuration that comes from remote server
		// env
		session := podmanTest.Podman([]string{"run", ALPINE, "printenv", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("bar"))

		// dns-search, server, options
		session = podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputStartsWith("search")).To(BeTrue())
		Expect(session.OutputToString()).To(ContainSubstring("foobar.com"))
		Expect(session.OutputToString()).To(ContainSubstring("1.2.3.4"))
		Expect(session.OutputToString()).To(ContainSubstring("debug"))

		// sysctls
		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "cat", "/proc/sys/net/ipv4/ping_group_range"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("1000"))

		// shm-size
		session = podmanTest.Podman([]string{"run", ALPINE, "grep", "shm", "/proc/self/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("size=200k"))

		// ulimits
		session = podmanTest.Podman([]string{"run", "--rm", fedoraMinimal, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("500"))

		// Configuration that comes from remote client
		// Timezone
		session = podmanTest.Podman([]string{"run", ALPINE, "date", "+'%H %Z'"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Or(ContainSubstring("EST"), ContainSubstring("EDT")))

		// Umask
		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "sh", "-c", "umask"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("0022"))
	})

	It("podman run containers.conf annotations test", func() {
		//containers.conf is set to   "run.oci.keep_original_groups=1"
		session := podmanTest.Podman([]string{"create", "--rm", "--name", "test", fedoraMinimal})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{ .Config.Annotations }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(ContainSubstring("run.oci.keep_original_groups:1"))
	})

	It("podman run with --add-host and no-hosts=true fails", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--add-host", "test1:127.0.0.1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("--no-hosts and --add-host cannot be set together"))

		session = podmanTest.Podman([]string{"run", "-dt", "--add-host", "test1:127.0.0.1", "--no-hosts=false", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run with no-hosts=true /etc/hosts does not include hostname", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--name", "test", ALPINE, "cat", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("test")))

		session = podmanTest.Podman([]string{"run", "--rm", "--name", "test", "--no-hosts=false", ALPINE, "cat", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("test"))
	})

	It("podman info seccomp profile path", func() {
		configPath := filepath.Join(podmanTest.TempDir, "containers.conf")
		os.Setenv("CONTAINERS_CONF", configPath)

		profile := filepath.Join(podmanTest.TempDir, "seccomp.json")
		containersConf := []byte(fmt.Sprintf("[containers]\nseccomp_profile=\"%s\"", profile))
		err = ioutil.WriteFile(configPath, containersConf, os.ModePerm)
		Expect(err).To(BeNil())

		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		session := podmanTest.Podman([]string{"info", "--format", "{{.Host.Security.SECCOMPProfilePath}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal(profile))
	})
})
