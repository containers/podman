package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Verify podman containers.conf usage", func() {
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

	It("limits test", func() {
		SkipIfRootlessCgroupsV1("Setting limits not supported on cgroupv1 for rootless users")
		// containers.conf is set to "nofile=500:500"
		session := podmanTest.Podman([]string{"run", "--rm", fedoraMinimal, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("500"))

		session = podmanTest.Podman([]string{"run", "--rm", "--ulimit", "nofile=2048:2048", fedoraMinimal, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("2048"))
	})

	It("having additional env", func() {
		// containers.conf default env includes foo
		session := podmanTest.Podman([]string{"run", ALPINE, "printenv"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("foo=bar"))
	})

	It("additional devices", func() {
		// containers.conf devices includes notone
		session := podmanTest.Podman([]string{"run", "--device", "/dev/null:/dev/bar", ALPINE, "ls", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(
			And(
				ContainSubstring("bar"),
				ContainSubstring("notone"),
			))
	})

	It("shm-size", func() {
		// containers.conf default sets shm-size=201k, which ends up as 200k
		session := podmanTest.Podman([]string{"run", ALPINE, "grep", "shm", "/proc/self/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("size=200k"))

		session = podmanTest.Podman([]string{"run", "--shm-size", "1g", ALPINE, "grep", "shm", "/proc/self/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("size=1048576k"))
	})

	It("add capabilities", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CGroupsV1")
		cap := podmanTest.Podman([]string{"run", ALPINE, "grep", "CapEff", "/proc/self/status"})
		cap.WaitWithDefaultTimeout()
		Expect(cap).Should(Exit(0))

		os.Setenv("CONTAINERS_CONF", "config/containers-ns.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		session := podmanTest.Podman([]string{"run", BB, "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).ToNot(Equal(cap.OutputToString()))
	})

	It("regular capabilities", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		result := podmanTest.Podman([]string{"top", "test1", "capeff"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.Out.Contents()).To(
			And(
				ContainSubstring("SYS_CHROOT"),
				ContainSubstring("NET_RAW"),
			))
	})

	It("drop capabilities", func() {
		os.Setenv("CONTAINERS_CONF", "config/containers-caps.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		result := podmanTest.Podman([]string{"container", "top", "test1", "capeff"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.Out.Contents()).ToNot(
			And(
				ContainSubstring("SYS_CHROOT"),
				ContainSubstring("NET_RAW"),
			))
	})

	verifyNSHandling := func(nspath, option string) {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		os.Setenv("CONTAINERS_CONF", "config/containers-ns.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		// containers.conf default ipcns to default to host
		session := podmanTest.Podman([]string{"run", ALPINE, "ls", "-l", nspath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		fields := strings.Split(session.OutputToString(), " ")
		ctrNS := strings.TrimSuffix(fields[len(fields)-1], "\n")

		cmd := exec.Command("ls", "-l", nspath)
		res, err := cmd.Output()
		Expect(err).ToNot(HaveOccurred())
		fields = strings.Split(string(res), " ")
		hostNS := strings.TrimSuffix(fields[len(fields)-1], "\n")
		Expect(hostNS).To(Equal(ctrNS))

		session = podmanTest.Podman([]string{"run", option, "private", ALPINE, "ls", "-l", nspath})
		fields = strings.Split(session.OutputToString(), " ")
		ctrNS = fields[len(fields)-1]
		Expect(hostNS).ToNot(Equal(ctrNS))
	}

	It("netns", func() {
		verifyNSHandling("/proc/self/ns/net", "--network")
	})

	It("ipcns", func() {
		verifyNSHandling("/proc/self/ns/ipc", "--ipc")
	})

	It("utsns", func() {
		verifyNSHandling("/proc/self/ns/uts", "--uts")
	})

	It("pidns", func() {
		verifyNSHandling("/proc/self/ns/pid", "--pid")
	})

	It("cgroupns", func() {
		verifyNSHandling("/proc/self/ns/cgroup", "--cgroupns")
	})

	It("using journald for container with container log_tag", func() {
		SkipIfInContainer("journalctl inside a container doesn't work correctly")
		os.Setenv("CONTAINERS_CONF", "config/containers-journald.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		logc := podmanTest.Podman([]string{"run", "-d", ALPINE, "sh", "-c", "echo podman; sleep 0.1; echo podman; sleep 0.1; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).Should(Exit(0))
		cid := logc.OutputToString()

		wait := podmanTest.Podman([]string{"wait", cid})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(Exit(0))

		cmd := exec.Command("journalctl", "--no-pager", "-o", "json", "--output-fields=CONTAINER_TAG", fmt.Sprintf("CONTAINER_ID_FULL=%s", cid))
		out, err := cmd.CombinedOutput()
		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(ContainSubstring("alpine"))
	})

	It("add volumes", func() {
		conffile := filepath.Join(podmanTest.TempDir, "container.conf")
		tempdir, err = CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())

		err := ioutil.WriteFile(conffile, []byte(fmt.Sprintf("[containers]\nvolumes=[\"%s:%s:Z\",]\n", tempdir, tempdir)), 0755)
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("CONTAINERS_CONF", conffile)
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		result := podmanTest.Podman([]string{"run", ALPINE, "ls", tempdir})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("sysctl test", func() {
		// containers.conf is set to   "net.ipv4.ping_group_range=0 1000"
		session := podmanTest.Podman([]string{"run", "--rm", fedoraMinimal, "cat", "/proc/sys/net/ipv4/ping_group_range"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("1000"))

		// Ignore containers.conf setting if --net=host
		session = podmanTest.Podman([]string{"run", "--rm", "--net", "host", fedoraMinimal, "cat", "/proc/sys/net/ipv4/ping_group_range"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).ToNot((ContainSubstring("1000")))
	})

	It("search domain", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("search foobar.com")))
	})

	It("add dns server", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("nameserver 1.2.3.4")))
	})

	It("add dns option", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("options debug")))
	})

	It("remove all search domain", func() {
		session := podmanTest.Podman([]string{"run", "--dns-search=.", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(Not(ContainElement(HavePrefix("search"))))
	})

	It("add search domain", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("search")))
		Expect(session.Out.Contents()).To(
			And(
				ContainSubstring("foobar.com"),
				ContainSubstring("1.2.3.4"),
				ContainSubstring("debug"),
			))
	})

	It("add timezone", func() {
		// containers.conf timezone set to Pacific/Honolulu
		session := podmanTest.Podman([]string{"run", "--tz", "", ALPINE, "date", "+'%H %Z'"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("HST"))

		// verify flag still overrides
		session = podmanTest.Podman([]string{"run", "--tz", "EST", ALPINE, "date", "+'%H %Z'"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("EST"))
	})

	It("add umask", func() {
		// containers.conf umask set to 0002
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test only works on crun")
		}

		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "sh", "-c", "umask"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("0002"))
	})

	It("network slirp options to allow host loopback", func() {
		session := podmanTest.Podman([]string{"run", "--network", "slirp4netns", ALPINE, "ping", "-c1", "10.0.2.2"})
		session.Wait(30)
		Expect(session).Should(Exit(0))
	})

	It("podman-remote test localcontainers.conf", func() {
		SkipIfNotRemote("this test is only for remote")

		os.Setenv("CONTAINERS_CONF", "config/containers-remote.conf")
		// Configuration that comes from remote server
		// env
		session := podmanTest.Podman([]string{"run", ALPINE, "printenv", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("bar"))

		// dns-search, server, options
		session = podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("search")))
		Expect(session.Out.Contents()).To(
			And(
				ContainSubstring("foobar.com"),
				ContainSubstring("1.2.3.4"),
				ContainSubstring("debug"),
			))

		// sysctls
		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "cat", "/proc/sys/net/ipv4/ping_group_range"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.Out.Contents()).To(ContainSubstring("1000"))

		// shm-size
		session = podmanTest.Podman([]string{"run", ALPINE, "grep", "shm", "/proc/self/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.Out.Contents()).To(ContainSubstring("size=200k"))

		// ulimits
		session = podmanTest.Podman([]string{"run", "--rm", fedoraMinimal, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.Out.Contents()).To(ContainSubstring("500"))

		// Configuration that comes from remote client
		// Timezone
		session = podmanTest.Podman([]string{"run", ALPINE, "date", "+'%H %Z'"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.Out.Contents()).To(
			Or(
				ContainSubstring("EST"),
				ContainSubstring("EDT"),
			))

		// Umask
		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "sh", "-c", "umask"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("0022"))
	})

	It("add annotations", func() {
		// containers.conf is set to   "run.oci.keep_original_groups=1"
		session := podmanTest.Podman([]string{"create", "--rm", "--name", "test", fedoraMinimal})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{ .Config.Annotations }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.Out.Contents()).To(ContainSubstring("run.oci.keep_original_groups:1"))
	})

	It("--add-host and no-hosts=true fails", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--add-host", "test1:127.0.0.1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.Err.Contents()).To(ContainSubstring("--no-hosts and --add-host cannot be set together"))

		session = podmanTest.Podman([]string{"run", "-dt", "--add-host", "test1:127.0.0.1", "--no-hosts=false", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("no-hosts=true /etc/hosts does not include hostname", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--name", "test", ALPINE, "cat", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.Out.Contents()).ToNot(ContainSubstring("test"))

		session = podmanTest.Podman([]string{"run", "--rm", "--name", "test", "--no-hosts=false", ALPINE, "cat", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.Out.Contents()).To(ContainSubstring("test"))
	})

	It("seccomp profile path", func() {
		configPath := filepath.Join(podmanTest.TempDir, "containers.conf")
		os.Setenv("CONTAINERS_CONF", configPath)

		profile := filepath.Join(podmanTest.TempDir, "seccomp.json")
		containersConf := []byte(fmt.Sprintf("[containers]\nseccomp_profile=\"%s\"", profile))
		err = ioutil.WriteFile(configPath, containersConf, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		session := podmanTest.Podman([]string{"info", "--format", "{{.Host.Security.SECCOMPProfilePath}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(profile))
	})

	It("add image_copy_tmp_dir", func() {
		session := podmanTest.Podman([]string{"info", "--format", "{{.Store.ImageCopyTmpDir}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("/var/tmp"))

		configPath := filepath.Join(podmanTest.TempDir, "containers.conf")
		os.Setenv("CONTAINERS_CONF", configPath)

		containersConf := []byte("[engine]\nimage_copy_tmp_dir=\"/foobar\"")
		err = ioutil.WriteFile(configPath, containersConf, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		session = podmanTest.Podman([]string{"info", "--format", "{{.Store.ImageCopyTmpDir}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("/foobar"))

		containersConf = []byte("[engine]\nimage_copy_tmp_dir=\"storage\"")
		err = ioutil.WriteFile(configPath, containersConf, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		session = podmanTest.Podman([]string{"info", "--format", "{{.Store.ImageCopyTmpDir}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.Out.Contents()).To(ContainSubstring("containers/storage/tmp"))

		containersConf = []byte("[engine]\nimage_copy_tmp_dir=\"storage1\"")
		err = ioutil.WriteFile(configPath, containersConf, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		session = podmanTest.Podman([]string{"info", "--format", "{{.Store.ImageCopyTmpDir}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.Err.Contents()).To(ContainSubstring("invalid image_copy_tmp_dir"))
	})

	// FIXME not sure why this is here
	It("system service --help shows (default 20)", func() {
		SkipIfRemote("system service is not supported on clients")

		result := podmanTest.Podman([]string{"system", "service", "--help"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.Out.Contents()).To(ContainSubstring("(default 1234)"))
	})

	It("bad infra_image name", func() {
		infra1 := "i.do/not/exist:image"
		infra2 := "i.still.do/not/exist:image"
		errorString := "initializing source docker://" + infra1
		error2String := "initializing source docker://" + infra2
		configPath := filepath.Join(podmanTest.TempDir, "containers.conf")
		os.Setenv("CONTAINERS_CONF", configPath)

		containersConf := []byte("[engine]\ninfra_image=\"" + infra1 + "\"")
		err = ioutil.WriteFile(configPath, containersConf, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		result := podmanTest.Podman([]string{"pod", "create", "--infra-image", infra2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.Err.Contents()).To(ContainSubstring(error2String))

		result = podmanTest.Podman([]string{"pod", "create"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.Err.Contents()).To(ContainSubstring(errorString))

		result = podmanTest.Podman([]string{"create", "--pod", "new:pod1", ALPINE})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.Err.Contents()).To(ContainSubstring(errorString))
	})

	It("set .engine.remote=true", func() {
		SkipIfRemote("only meaningful when running ABI/local")

		// Need to restore CONTAINERS_CONF or AfterEach() will fail
		if path, found := os.LookupEnv("CONTAINERS_CONF"); found {
			defer os.Setenv("CONTAINERS_CONF", path)
		}

		configPath := filepath.Join(podmanTest.TempDir, "containers-engine-remote.conf")
		os.Setenv("CONTAINERS_CONF", configPath)
		defer os.Remove(configPath)

		err := ioutil.WriteFile(configPath, []byte("[engine]\nremote=true"), os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		// podmanTest.Podman() cannot be used as it was initialized remote==false
		cmd := exec.Command(podmanTest.PodmanBinary, "info", "--format", "{{.Host.ServiceIsRemote}}")
		session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())

		description := "Should have failed as there is no running remote API service available."
		Eventually(session, DefaultWaitTimeout).Should(Exit(125), description)
		Expect(session.Err).Should(Say("Error: unable to connect to Podman socket"))
	})

	It("podman containers.conf cgroups=disabled", func() {
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("FIXME: requires crun")
		}

		conffile := filepath.Join(podmanTest.TempDir, "container.conf")
		err := ioutil.WriteFile(conffile, []byte("[containers]\ncgroups=\"disabled\"\n"), 0755)
		Expect(err).ToNot(HaveOccurred())

		result := podmanTest.Podman([]string{"create", ALPINE, "true"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{ .HostConfig.Cgroups }}", result.OutputToString()})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).ToNot(Equal("disabled"))

		os.Setenv("CONTAINERS_CONF", conffile)
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		result = podmanTest.Podman([]string{"create", ALPINE, "true"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		inspect = podmanTest.Podman([]string{"inspect", "--format", "{{ .HostConfig.Cgroups }}", result.OutputToString()})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("disabled"))
	})

	It("podman containers.conf runtime", func() {
		SkipIfRemote("--runtime option is not available for remote commands")
		conffile := filepath.Join(podmanTest.TempDir, "container.conf")
		err := ioutil.WriteFile(conffile, []byte("[engine]\nruntime=\"testruntime\"\n"), 0755)
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("CONTAINERS_CONF", conffile)
		result := podmanTest.Podman([]string{"--help"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring("Path to the OCI-compatible binary used to run containers. (default \"testruntime\")"))
	})
})
