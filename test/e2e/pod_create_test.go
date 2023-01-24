package integration

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/common/pkg/apparmor"
	"github.com/containers/common/pkg/seccomp"
	"github.com/containers/common/pkg/sysinfo"
	"github.com/containers/podman/v4/pkg/util"
	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/opencontainers/selinux/go-selinux"
)

var _ = Describe("Podman pod create", func() {
	var (
		tempdir     string
		err         error
		podmanTest  *PodmanTestIntegration
		hostname, _ = os.Hostname()
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman create pod", func() {
		_, ec, podID := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToString()).To(ContainSubstring(podID))
		Expect(check.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman create pod with name", func() {
		name := "test"
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {name}})
		Expect(ec).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToString()).To(ContainSubstring(name))
	})

	It("podman create pod with doubled name", func() {
		name := "test"
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {name}})
		Expect(ec).To(Equal(0))

		_, ec2, _ := podmanTest.CreatePod(map[string][]string{"--name": {name}})
		Expect(ec2).To(Not(Equal(0)))

		check := podmanTest.Podman([]string{"pod", "ps", "-q"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman create pod with same name as ctr", func() {
		name := "test"
		session := podmanTest.Podman([]string{"create", "--name", name, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {name}})
		Expect(ec).To(Not(Equal(0)))

		check := podmanTest.Podman([]string{"pod", "ps", "-q"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToStringArray()).To(BeEmpty())
	})

	It("podman create pod without network portbindings", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		pod := session.OutputToString()

		webserver := podmanTest.Podman([]string{"run", "--pod", pod, "-dt", NGINX_IMAGE})
		webserver.WaitWithDefaultTimeout()
		Expect(webserver).Should(Exit(0))

		check := SystemExec("nc", []string{"-z", "localhost", "80"})
		Expect(check).Should(Exit(1))
	})

	It("podman create pod with network portbindings", func() {
		name := "test"
		port := GetPort()
		session := podmanTest.Podman([]string{"pod", "create", "--name", name, "-p", fmt.Sprintf("%d:80", port)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		pod := session.OutputToString()

		webserver := podmanTest.Podman([]string{"run", "--pod", pod, "-dt", NGINX_IMAGE})
		webserver.WaitWithDefaultTimeout()
		Expect(webserver).Should(Exit(0))
		Expect(ncz(port)).To(BeTrue())
	})

	It("podman create pod with id file with network portbindings", func() {
		file := filepath.Join(podmanTest.TempDir, "pod.id")
		name := "test"
		port := GetPort()
		session := podmanTest.Podman([]string{"pod", "create", "--name", name, "--pod-id-file", file, "-p", fmt.Sprintf("%d:80", port)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		webserver := podmanTest.Podman([]string{"run", "--pod-id-file", file, "-dt", NGINX_IMAGE})
		webserver.WaitWithDefaultTimeout()
		Expect(webserver).Should(Exit(0))
		Expect(ncz(port)).To(BeTrue())
	})

	It("podman create pod with no infra but portbindings should fail", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--infra=false", "--name", name, "-p", "80:80"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman create pod with --no-hosts", func() {
		name := "test"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--no-hosts", "--name", name})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		alpineResolvConf := podmanTest.Podman([]string{"run", "-ti", "--rm", "--no-hosts", ALPINE, "cat", "/etc/hosts"})
		alpineResolvConf.WaitWithDefaultTimeout()
		Expect(alpineResolvConf).Should(Exit(0))

		podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "-ti", "--rm", ALPINE, "cat", "/etc/hosts"})
		podResolvConf.WaitWithDefaultTimeout()
		Expect(podResolvConf).Should(Exit(0))
		Expect(podResolvConf.OutputToString()).To(Equal(alpineResolvConf.OutputToString()))
	})

	It("podman create pod with --no-hosts and no infra should fail", func() {
		name := "test"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--no-hosts", "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(125))
	})

	It("podman create pod with --add-host", func() {
		name := "test"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--add-host", "test.example.com:12.34.56.78", "--name", name})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "-ti", "--rm", ALPINE, "cat", "/etc/hosts"})
		podResolvConf.WaitWithDefaultTimeout()
		Expect(podResolvConf).Should(Exit(0))
		Expect(podResolvConf.OutputToString()).To(ContainSubstring("12.34.56.78 test.example.com"))
	})

	It("podman create pod with --add-host and no infra should fail", func() {
		name := "test"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--add-host", "test.example.com:12.34.56.78", "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(125))
	})

	It("podman create pod with DNS server set", func() {
		name := "test"
		server := "12.34.56.78"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--dns", server, "--name", name})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "-ti", "--rm", ALPINE, "cat", "/etc/resolv.conf"})
		podResolvConf.WaitWithDefaultTimeout()
		Expect(podResolvConf).Should(Exit(0))
		Expect(podResolvConf.OutputToString()).To(ContainSubstring("nameserver %s", server))
	})

	It("podman create pod with DNS server set and no infra should fail", func() {
		name := "test"
		server := "12.34.56.78"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--dns", server, "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(125))
	})

	It("podman create pod with DNS option set", func() {
		name := "test"
		option := "attempts:5"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--dns-opt", option, "--name", name})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "-ti", "--rm", ALPINE, "cat", "/etc/resolv.conf"})
		podResolvConf.WaitWithDefaultTimeout()
		Expect(podResolvConf).Should(Exit(0))
		Expect(podResolvConf.OutputToString()).To(ContainSubstring(fmt.Sprintf("options %s", option)))
	})

	It("podman create pod with DNS option set and no infra should fail", func() {
		name := "test"
		option := "attempts:5"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--dns-opt", option, "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(125))
	})

	It("podman create pod with DNS search domain set", func() {
		name := "test"
		search := "example.com"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--dns-search", search, "--name", name})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "-ti", "--rm", ALPINE, "cat", "/etc/resolv.conf"})
		podResolvConf.WaitWithDefaultTimeout()
		Expect(podResolvConf).Should(Exit(0))
		Expect(podResolvConf.OutputToString()).To(ContainSubstring(fmt.Sprintf("search %s", search)))
	})

	It("podman create pod with DNS search domain set and no infra should fail", func() {
		name := "test"
		search := "example.com"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--dns-search", search, "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(125))
	})

	It("podman create pod with IP address", func() {
		name := "test"
		ip := GetRandomIPAddress()
		podCreate := podmanTest.Podman([]string{"pod", "create", "--ip", ip, "--name", name})
		podCreate.WaitWithDefaultTimeout()
		// Rootless should error without network
		if isRootless() {
			Expect(podCreate).Should(Exit(125))
		} else {
			Expect(podCreate).Should(Exit(0))
			podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "-ti", "--rm", ALPINE, "ip", "addr"})
			podResolvConf.WaitWithDefaultTimeout()
			Expect(podResolvConf).Should(Exit(0))
			Expect(podResolvConf.OutputToString()).To(ContainSubstring(ip))
		}
	})

	It("podman container in pod with IP address shares IP address", func() {
		SkipIfRootless("Rootless does not support --ip without network")
		podName := "test"
		ctrName := "testCtr"
		ip := GetRandomIPAddress()
		podCreate := podmanTest.Podman([]string{"pod", "create", "--ip", ip, "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		podCtr := podmanTest.Podman([]string{"run", "--name", ctrName, "--pod", podName, "-d", "-t", ALPINE, "top"})
		podCtr.WaitWithDefaultTimeout()
		Expect(podCtr).Should(Exit(0))
		ctrInspect := podmanTest.Podman([]string{"inspect", ctrName})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(Exit(0))
		ctrJSON := ctrInspect.InspectContainerToJSON()
		Expect(ctrJSON[0].NetworkSettings).To(HaveField("IPAddress", ip))
	})

	It("podman create pod with IP address and no infra should fail", func() {
		name := "test"
		ip := GetRandomIPAddress()
		podCreate := podmanTest.Podman([]string{"pod", "create", "--ip", ip, "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(125))
	})

	It("podman create pod with MAC address", func() {
		name := "test"
		mac := "92:d0:c6:0a:29:35"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--mac-address", mac, "--name", name})
		podCreate.WaitWithDefaultTimeout()
		// Rootless should error
		if isRootless() {
			Expect(podCreate).Should(Exit(125))
		} else {
			Expect(podCreate).Should(Exit(0))
			podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "-ti", "--rm", ALPINE, "ip", "addr"})
			podResolvConf.WaitWithDefaultTimeout()
			Expect(podResolvConf).Should(Exit(0))
			Expect(podResolvConf.OutputToString()).To(ContainSubstring(mac))
		}
	})

	It("podman create pod with MAC address and no infra should fail", func() {
		name := "test"
		mac := "92:d0:c6:0a:29:35"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--mac-address", mac, "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(125))
	})

	It("podman create pod and print id to external file", func() {
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(os.TempDir())).To(Succeed())
		targetPath, err := CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		targetFile := filepath.Join(targetPath, "idFile")
		defer Expect(os.RemoveAll(targetFile)).To(BeNil())
		defer Expect(os.Chdir(cwd)).To(BeNil())

		session := podmanTest.Podman([]string{"pod", "create", "--name=abc", "--pod-id-file", targetFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		id, _ := os.ReadFile(targetFile)
		check := podmanTest.Podman([]string{"pod", "inspect", "abc"})
		check.WaitWithDefaultTimeout()
		data := check.InspectPodToJSON()
		Expect(data).To(HaveField("ID", string(id)))
	})

	It("podman pod create --replace", func() {
		// Make sure we error out with --name.
		session := podmanTest.Podman([]string{"pod", "create", "--replace", ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))

		// Create and replace 5 times in a row the "same" pod.
		podName := "testCtr"
		for i := 0; i < 5; i++ {
			session = podmanTest.Podman([]string{"pod", "create", "--replace", "--name", podName})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}
	})

	It("podman create pod with defaults", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"pod", "inspect", name})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(Exit(0))
		data := check.InspectPodToJSON()

		check1 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Config.Entrypoint}}", data.Containers[0].ID})
		check1.WaitWithDefaultTimeout()
		Expect(check1).Should(Exit(0))
		Expect(check1.OutputToString()).To(Equal("/catatonit -P"))

		// check the Path and Args
		check2 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Path}}:{{.Args}}", data.Containers[0].ID})
		check2.WaitWithDefaultTimeout()
		Expect(check2).Should(Exit(0))
		Expect(check2.OutputToString()).To(Equal("/catatonit:[-P]"))
	})

	It("podman create pod with --infra-command", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--infra-command", "/pause1", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"pod", "inspect", name})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(Exit(0))
		data := check.InspectPodToJSON()

		check1 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Config.Entrypoint}}", data.Containers[0].ID})
		check1.WaitWithDefaultTimeout()
		Expect(check1).Should(Exit(0))
		Expect(check1.OutputToString()).To(Equal("/pause1"))

		// check the Path and Args
		check2 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Path}}:{{.Args}}", data.Containers[0].ID})
		check2.WaitWithDefaultTimeout()
		Expect(check2).Should(Exit(0))
		Expect(check2.OutputToString()).To(Equal("/pause1:[/pause1]"))
	})

	It("podman create pod with --infra-image", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
entrypoint ["/fromimage"]
`
		podmanTest.BuildImage(dockerfile, "localhost/infra", "false")
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--infra-image", "localhost/infra", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"pod", "inspect", name})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(Exit(0))
		data := check.InspectPodToJSON()

		check1 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Config.Entrypoint}}", data.Containers[0].ID})
		check1.WaitWithDefaultTimeout()
		Expect(check1).Should(Exit(0))
		Expect(check1.OutputToString()).To(Equal("/fromimage"))

		// check the Path and Args
		check2 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Path}}:{{.Args}}", data.Containers[0].ID})
		check2.WaitWithDefaultTimeout()
		Expect(check2).Should(Exit(0))
		Expect(check2.OutputToString()).To(Equal("/fromimage:[/fromimage]"))
	})

	It("podman create pod with --infra-command --infra-image", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
entrypoint ["/fromimage"]
`
		podmanTest.BuildImage(dockerfile, "localhost/infra", "false")
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--infra-image", "localhost/infra", "--infra-command", "/fromcommand", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"pod", "inspect", name})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(Exit(0))
		data := check.InspectPodToJSON()

		check1 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Config.Entrypoint}}", data.Containers[0].ID})
		check1.WaitWithDefaultTimeout()
		Expect(check1).Should(Exit(0))
		Expect(check1.OutputToString()).To(Equal("/fromcommand"))

		// check the Path and Args
		check2 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Path}}:{{.Args}}", data.Containers[0].ID})
		check2.WaitWithDefaultTimeout()
		Expect(check2).Should(Exit(0))
		Expect(check2.OutputToString()).To(Equal("/fromcommand:[/fromcommand]"))
	})

	It("podman create pod with slirp network option", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--name", name, "--network", "slirp4netns:port_handler=slirp4netns", "-p", "8082:8000"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"pod", "inspect", "--format", "{{.InfraConfig.NetworkOptions.slirp4netns}}", name})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(Exit(0))
		Expect(check.OutputToString()).To(Equal("[port_handler=slirp4netns]"))
	})

	It("podman pod status test", func() {
		podName := "testpod"
		create := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		status1 := podmanTest.Podman([]string{"pod", "inspect", "--format", "{{ .State }}", podName})
		status1.WaitWithDefaultTimeout()
		Expect(status1).Should(Exit(0))
		Expect(status1.OutputToString()).To(ContainSubstring("Created"))

		ctr1 := podmanTest.Podman([]string{"run", "--pod", podName, "-d", ALPINE, "top"})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(Exit(0))

		status2 := podmanTest.Podman([]string{"pod", "inspect", "--format", "{{ .State }}", podName})
		status2.WaitWithDefaultTimeout()
		Expect(status2).Should(Exit(0))
		Expect(status2.OutputToString()).To(ContainSubstring("Running"))

		ctr2 := podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(Exit(0))

		status3 := podmanTest.Podman([]string{"pod", "inspect", "--format", "{{ .State }}", podName})
		status3.WaitWithDefaultTimeout()
		Expect(status3).Should(Exit(0))
		Expect(status3.OutputToString()).To(ContainSubstring("Degraded"))
	})

	It("podman create with unsupported network options", func() {
		podCreate := podmanTest.Podman([]string{"pod", "create", "--network", "container:doesnotmatter"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(125))
		Expect(podCreate.ErrorToString()).To(ContainSubstring("pods presently do not support network mode container"))
	})

	It("podman pod create with namespace path networking", func() {
		SkipIfRootless("ip netns is not supported for rootless users")
		SkipIfContainerized("ip netns cannot be run within a container.")

		podName := "netnspod"
		netNsName := "test1"
		networkMode := fmt.Sprintf("ns:/var/run/netns/%s", netNsName)

		addNetns := SystemExec("ip", []string{"netns", "add", netNsName})
		Expect(addNetns).Should(Exit(0))
		defer func() {
			delNetns := SystemExec("ip", []string{"netns", "delete", netNsName})
			Expect(delNetns).Should(Exit(0))
		}()

		podCreate := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--network", networkMode})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podStart := podmanTest.Podman([]string{"pod", "start", podName})
		podStart.WaitWithDefaultTimeout()
		Expect(podStart).Should(Exit(0))

		inspectPod := podmanTest.Podman([]string{"pod", "inspect", podName})
		inspectPod.WaitWithDefaultTimeout()
		Expect(inspectPod).Should(Exit(0))
		inspectPodJSON := inspectPod.InspectPodToJSON()

		inspectInfraContainer := podmanTest.Podman([]string{"inspect", inspectPodJSON.InfraContainerID})
		inspectInfraContainer.WaitWithDefaultTimeout()
		Expect(inspectInfraContainer).Should(Exit(0))
		inspectInfraContainerJSON := inspectInfraContainer.InspectContainerToJSON()

		Expect(inspectInfraContainerJSON[0].HostConfig.NetworkMode).To(Equal(networkMode))
	})

	It("podman pod create with --net=none", func() {
		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--network", "none", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "ip", "-o", "-4", "addr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("inet 127.0.0.1/8 scope host lo"))
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman pod create --infra-image w/untagged image", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		dockerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT ["sleep","99999"]
		`
		// This builds a none/none image
		iid := podmanTest.BuildImage(dockerfile, "", "true")

		create := podmanTest.Podman([]string{"pod", "create", "--infra-image", iid})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))
	})

	It("podman pod create --cpus", func() {
		podName := "testPod"
		numCPU := float64(sysinfo.NumCPU())
		period, quota := util.CoresToPeriodAndQuota(numCPU)
		numCPUStr := strconv.Itoa(int(numCPU))
		podCreate := podmanTest.Podman([]string{"pod", "create", "--cpus", numCPUStr, "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		contCreate := podmanTest.Podman([]string{"container", "create", "--pod", podName, "alpine"})
		contCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(Exit(0))
		podJSON := podInspect.InspectPodToJSON()
		Expect(podJSON).To(HaveField("CPUPeriod", period))
		Expect(podJSON).To(HaveField("CPUQuota", quota))
	})

	It("podman pod create --cpuset-cpus", func() {
		podName := "testPod"
		ctrName := "testCtr"
		numCPU := float64(sysinfo.NumCPU()) - 1
		numCPUStr := strconv.Itoa(int(numCPU))
		in := "0-" + numCPUStr
		podCreate := podmanTest.Podman([]string{"pod", "create", "--cpuset-cpus", in, "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		contCreate := podmanTest.Podman([]string{"container", "create", "--name", ctrName, "--pod", podName, "alpine"})
		contCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(Exit(0))
		podJSON := podInspect.InspectPodToJSON()
		Expect(podJSON).To(HaveField("CPUSetCPUs", in))
	})

	It("podman pod create --pid", func() {
		podName := "pidPod"
		ns := "ns:/proc/self/ns/"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--pid", ns, "--name", podName, "--share", "pid"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(Exit(0))
		podJSON := podInspect.InspectPodToJSON()
		Expect(podJSON.InfraConfig).To(HaveField("PidNS", ns))

		podName = "pidPod2"
		ns = "pod"

		podCreate = podmanTest.Podman([]string{"pod", "create", "--pid", ns, "--name", podName, "--share", "pid"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError())

		podName = "pidPod3"
		ns = "host"

		podCreate = podmanTest.Podman([]string{"pod", "create", "--pid", ns, "--name", podName, "--share", "pid"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podInspect = podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(Exit(0))
		podJSON = podInspect.InspectPodToJSON()
		Expect(podJSON.InfraConfig).To(HaveField("PidNS", "host"))

		podName = "pidPod4"
		ns = "private"

		podCreate = podmanTest.Podman([]string{"pod", "create", "--pid", ns, "--name", podName, "--share", "pid"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podInspect = podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(Exit(0))
		podJSON = podInspect.InspectPodToJSON()
		Expect(podJSON.InfraConfig).To(HaveField("PidNS", "private"))

		podName = "pidPod5"
		ns = "container:randomfakeid"

		podCreate = podmanTest.Podman([]string{"pod", "create", "--pid", ns, "--name", podName, "--share", "pid"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError())

	})

	It("podman pod create with --userns=keep-id", func() {
		if !isRootless() {
			Skip("Test only runs without root")
		}

		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--userns", "keep-id", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "id", "-u"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		uid := fmt.Sprintf("%d", os.Geteuid())
		Expect(session.OutputToString()).To(ContainSubstring(uid))

		// Check passwd
		session = podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "id", "-un"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		Expect(session.OutputToString()).To(ContainSubstring(u.Name))

		// root owns /usr
		session = podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "stat", "-c%u", "/usr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("0"))

		// fail if --pod and --userns set together
		session = podmanTest.Podman([]string{"run", "--pod", podName, "--userns", "keep-id", ALPINE, "id", "-u"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman pod create with --userns=keep-id can add users", func() {
		if !isRootless() {
			Skip("Test only runs without root")
		}

		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--userns", "keep-id", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		ctrName := "ctr-name"
		session := podmanTest.Podman([]string{"run", "--pod", podName, "-d", "--stop-signal", "9", "--name", ctrName, fedoraMinimal, "sleep", "600"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		// container inside pod inherits user from infra container if --user is not set
		// etc/passwd entry will look like USERNAME:*:1000:1000:Full User Name:/:/bin/sh
		exec1 := podmanTest.Podman([]string{"exec", ctrName, "cat", "/etc/passwd"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(Exit(0))
		Expect(exec1.OutputToString()).To(ContainSubstring(u.Name))

		exec2 := podmanTest.Podman([]string{"exec", ctrName, "useradd", "testuser"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(Exit(0))

		exec3 := podmanTest.Podman([]string{"exec", ctrName, "cat", "/etc/passwd"})
		exec3.WaitWithDefaultTimeout()
		Expect(exec3).Should(Exit(0))
		Expect(exec3.OutputToString()).To(ContainSubstring("testuser"))
	})

	It("podman pod create with --userns=auto", func() {
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		name := u.Name
		if name == "root" {
			name = "containers"
		}

		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		m := make(map[string]string)
		for i := 0; i < 5; i++ {
			podName := "testPod" + strconv.Itoa(i)
			podCreate := podmanTest.Podman([]string{"pod", "create", "--userns=auto", "--name", podName})
			podCreate.WaitWithDefaultTimeout()
			Expect(podCreate).Should(Exit(0))

			session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/uid_map"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			l := session.OutputToString()
			Expect(l).To(ContainSubstring("1024"))
			m[l] = l
		}
		// check for no duplicates
		Expect(m).To(HaveLen(5))
	})

	It("podman pod create --userns=auto:size=%d", func() {
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())

		name := u.Name
		if name == "root" {
			name = "containers"
		}

		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--userns=auto:size=500", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("500"))

		podName = "testPod-1"
		podCreate = podmanTest.Podman([]string{"pod", "create", "--userns=auto:size=3000", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		session = podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("3000"))
	})

	It("podman pod create --userns=auto:uidmapping=", func() {
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())

		name := u.Name
		if name == "root" {
			name = "containers"
		}

		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--userns=auto:uidmapping=0:0:1", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output := session.OutputToString()
		Expect(output).To(MatchRegexp("\\s0\\s0\\s1"))

		podName = "testPod-1"
		podCreate = podmanTest.Podman([]string{"pod", "create", "--userns=auto:size=8192,uidmapping=0:0:1", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		session = podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("8191"))
	})

	It("podman pod create --userns=auto:gidmapping=", func() {
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())

		name := u.Name
		if name == "root" {
			name = "containers"
		}

		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--userns=auto:gidmapping=0:0:1", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/gid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output := session.OutputToString()
		Expect(output).To(MatchRegexp("\\s0\\s0\\s1"))

		podName = "testPod-1"
		podCreate = podmanTest.Podman([]string{"pod", "create", "--userns=auto:size=8192,gidmapping=0:0:1", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		session = podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/gid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("8191"))
	})

	It("podman pod create --volume", func() {
		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(Exit(0))
		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--volume", volName + ":/tmp1", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(Exit(0))
		data := podInspect.InspectPodToJSON()
		Expect(data.Mounts[0]).To(HaveField("Name", volName))
		ctrName := "testCtr"
		ctrCreate := podmanTest.Podman([]string{"create", "--pod", podName, "--name", ctrName, ALPINE})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(Exit(0))
		ctrInspect := podmanTest.Podman([]string{"inspect", ctrName})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(Exit(0))
		ctrData := ctrInspect.InspectContainerToJSON()
		Expect(ctrData[0].Mounts[0]).To(HaveField("Name", volName))

		ctr2 := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "sh", "-c", "echo hello >> " + "/tmp1/test"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(Exit(0))

		ctr3 := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/tmp1/test"})
		ctr3.WaitWithDefaultTimeout()
		Expect(ctr3.OutputToString()).To(ContainSubstring("hello"))

		ctr4 := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "touch", "/tmp1/testing.txt"})
		ctr4.WaitWithDefaultTimeout()
		Expect(ctr4).Should(Exit(0))
	})

	It("podman pod create --device", func() {
		SkipIfRootless("Cannot create devices in /dev in rootless mode")
		Expect(os.MkdirAll("/dev/foodevdir", os.ModePerm)).To(Succeed())
		defer os.RemoveAll("/dev/foodevdir")

		mknod := SystemExec("mknod", []string{"/dev/foodevdir/null", "c", "1", "3"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(Exit(0))

		podName := "testPod"
		session := podmanTest.Podman([]string{"pod", "create", "--device", "/dev/foodevdir:/dev/bar", "--name", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"run", "-q", "--pod", podName, ALPINE, "stat", "-c%t:%T", "/dev/bar/null"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("1:3"))

	})

	It("podman pod create --volumes-from", func() {
		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(Exit(0))
		ctrName := "testCtr"
		ctrCreate := podmanTest.Podman([]string{"create", "--volume", volName + ":/tmp1", "--name", ctrName, ALPINE})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(Exit(0))
		ctrInspect := podmanTest.Podman([]string{"inspect", ctrName})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(Exit(0))
		data := ctrInspect.InspectContainerToJSON()
		Expect(data[0].Mounts[0]).To(HaveField("Name", volName))
		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--volumes-from", ctrName, "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(Exit(0))
		podData := podInspect.InspectPodToJSON()
		Expect(podData.Mounts[0]).To(HaveField("Name", volName))

		ctr2 := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "sh", "-c", "echo hello >> " + "/tmp1/test"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(Exit(0))

		ctr3 := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/tmp1/test"})
		ctr3.WaitWithDefaultTimeout()
		Expect(ctr3.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman pod create read network mode from config", func() {
		confPath, err := filepath.Abs("config/containers-netns.conf")
		Expect(err).ToNot(HaveOccurred())
		os.Setenv("CONTAINERS_CONF", confPath)
		defer os.Unsetenv("CONTAINERS_CONF")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		pod := podmanTest.Podman([]string{"pod", "create", "--name", "test", "--infra-name", "test-infra"})
		pod.WaitWithDefaultTimeout()
		Expect(pod).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.HostConfig.NetworkMode}}", "test-infra"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).Should(Equal("host"))
	})

	It("podman pod create --security-opt", func() {
		if !selinux.GetEnabled() {
			Skip("SELinux not enabled")
		}
		podCreate := podmanTest.Podman([]string{"pod", "create", "--security-opt", "label=type:spc_t", "--security-opt", "seccomp=unconfined"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		ctrCreate := podmanTest.Podman([]string{"container", "create", "--pod", podCreate.OutputToString(), ALPINE})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(Exit(0))

		ctrInspect := podmanTest.InspectContainer(ctrCreate.OutputToString())
		Expect(ctrInspect[0].HostConfig).To(HaveField("SecurityOpt", []string{"label=type:spc_t", "seccomp=unconfined"}))

		podCreate = podmanTest.Podman([]string{"pod", "create", "--security-opt", "label=disable"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		ctrCreate = podmanTest.Podman([]string{"container", "run", "-it", "--pod", podCreate.OutputToString(), ALPINE, "cat", "/proc/self/attr/current"})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(Exit(0))
		match, _ := ctrCreate.GrepString("spc_t")
		Expect(match).Should(BeTrue())
	})

	It("podman pod create --security-opt seccomp", func() {
		if !seccomp.IsEnabled() {
			Skip("seccomp is not enabled")
		}
		podCreate := podmanTest.Podman([]string{"pod", "create", "--security-opt", "seccomp=unconfined"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		ctrCreate := podmanTest.Podman([]string{"container", "create", "--pod", podCreate.OutputToString(), ALPINE})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(Exit(0))

		ctrInspect := podmanTest.InspectContainer(ctrCreate.OutputToString())
		Expect(ctrInspect[0].HostConfig).To(HaveField("SecurityOpt", []string{"seccomp=unconfined"}))
	})

	It("podman pod create --security-opt apparmor test", func() {
		if !apparmor.IsEnabled() {
			Skip("Apparmor is not enabled")
		}
		podCreate := podmanTest.Podman([]string{"pod", "create", "--security-opt", fmt.Sprintf("apparmor=%s", apparmor.Profile)})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		ctrCreate := podmanTest.Podman([]string{"container", "create", "--pod", podCreate.OutputToString(), ALPINE})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(Exit(0))

		inspect := podmanTest.InspectContainer(ctrCreate.OutputToString())
		Expect(inspect[0]).To(HaveField("AppArmorProfile", apparmor.Profile))

	})

	It("podman pod create --sysctl test", func() {
		SkipIfRootless("Network sysctls are not available root rootless")
		podCreate := podmanTest.Podman([]string{"pod", "create", "--sysctl", "net.core.somaxconn=65535"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		session := podmanTest.Podman([]string{"run", "--pod", podCreate.OutputToString(), "--rm", ALPINE, "sysctl", "net.core.somaxconn"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("net.core.somaxconn = 65535"))

		// if not sharing the net NS, nothing should fail, but the sysctl should not be passed
		podCreate = podmanTest.Podman([]string{"pod", "create", "--share", "pid", "--sysctl", "net.core.somaxconn=65535"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		session = podmanTest.Podman([]string{"run", "--pod", podCreate.OutputToString(), "--rm", ALPINE, "sysctl", "net.core.somaxconn"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).NotTo(ContainSubstring("net.core.somaxconn = 65535"))

		// one other misc option
		podCreate = podmanTest.Podman([]string{"pod", "create", "--sysctl", "kernel.msgmax=65535"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		session = podmanTest.Podman([]string{"run", "--pod", podCreate.OutputToString(), "--rm", ALPINE, "sysctl", "kernel.msgmax"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("kernel.msgmax = 65535"))

		podCreate = podmanTest.Podman([]string{"pod", "create", "--share", "pid", "--sysctl", "kernel.msgmax=65535"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		session = podmanTest.Podman([]string{"run", "--pod", podCreate.OutputToString(), "--rm", ALPINE, "sysctl", "kernel.msgmax"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).NotTo(ContainSubstring("kernel.msgmax = 65535"))

	})

	It("podman pod create --share-parent test", func() {
		SkipIfRootlessCgroupsV1("rootless cannot use cgroups with cgroupsv1")
		SkipIfCgroupV1("FIXME: #15013: CgroupMode shows 'host' instead of CID under Cgroups V1.")
		podCreate := podmanTest.Podman([]string{"pod", "create", "--share-parent=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		ctrCreate := podmanTest.Podman([]string{"run", "-dt", "--pod", podCreate.OutputToString(), ALPINE})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(Exit(0))

		inspectPod := podmanTest.Podman([]string{"pod", "inspect", podCreate.OutputToString()})
		inspectPod.WaitWithDefaultTimeout()
		Expect(inspectPod).Should(Exit(0))
		data := inspectPod.InspectPodToJSON()

		inspect := podmanTest.InspectContainer(ctrCreate.OutputToString())
		Expect(data.CgroupPath).To(HaveLen(0))
		if podmanTest.CgroupManager == "cgroupfs" || !isRootless() {
			Expect(inspect[0].HostConfig.CgroupParent).To(HaveLen(0))
		} else if podmanTest.CgroupManager == "systemd" {
			Expect(inspect[0].HostConfig).To(HaveField("CgroupParent", "user.slice"))
		}

		podCreate2 := podmanTest.Podman([]string{"pod", "create", "--share", "cgroup,ipc,net,uts", "--share-parent=false", "--infra-name", "cgroupCtr"})
		podCreate2.WaitWithDefaultTimeout()
		Expect(podCreate2).Should(Exit(0))

		ctrCreate2 := podmanTest.Podman([]string{"run", "-dt", "--pod", podCreate2.OutputToString(), ALPINE})
		ctrCreate2.WaitWithDefaultTimeout()
		Expect(ctrCreate2).Should(Exit(0))

		inspectInfra := podmanTest.InspectContainer("cgroupCtr")

		inspect2 := podmanTest.InspectContainer(ctrCreate2.OutputToString())

		Expect(inspect2[0].HostConfig.CgroupMode).To(ContainSubstring(inspectInfra[0].ID))

		podCreate3 := podmanTest.Podman([]string{"pod", "create", "--share", "cgroup"})
		podCreate3.WaitWithDefaultTimeout()
		Expect(podCreate3).ShouldNot(Exit(0))

	})

	It("podman pod create infra inheritance test", func() {
		volName := "testVol1"
		volCreate := podmanTest.Podman([]string{"volume", "create", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(Exit(0))

		session := podmanTest.Podman([]string{"pod", "create", "-v", volName + ":/vol1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		volName2 := "testVol2"
		volCreate = podmanTest.Podman([]string{"volume", "create", volName2})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--pod", session.OutputToString(), "-v", volName2 + ":/vol2", ALPINE, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).Should(ContainSubstring("/vol1"))
		Expect(session.OutputToString()).Should(ContainSubstring("/vol2"))
	})

	It("podman pod create --shm-size", func() {
		podCreate := podmanTest.Podman([]string{"pod", "create", "--shm-size", "10mb"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		run := podmanTest.Podman([]string{"run", "-it", "--pod", podCreate.OutputToString(), ALPINE, "mount"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		t, strings := run.GrepString("shm on /dev/shm type tmpfs")
		Expect(t).To(BeTrue())
		Expect(strings[0]).Should(ContainSubstring("size=10240k"))
	})

	It("podman pod create --shm-size and --ipc=host conflict", func() {
		podCreate := podmanTest.Podman([]string{"pod", "create", "--shm-size", "10mb"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		run := podmanTest.Podman([]string{"run", "-dt", "--pod", podCreate.OutputToString(), "--ipc", "host", ALPINE})
		run.WaitWithDefaultTimeout()
		Expect(run).ShouldNot(Exit(0))
	})

	It("podman pod create --uts test", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--uts", "host"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "-it", "--pod", session.OutputToString(), ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(hostname))

		podName := "utsPod"
		ns := "ns:/proc/self/ns/"

		// just share uts with a custom path
		podCreate := podmanTest.Podman([]string{"pod", "create", "--uts", ns, "--name", podName, "--share", "uts"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(Exit(0))
		podJSON := podInspect.InspectPodToJSON()
		Expect(podJSON.InfraConfig).To(HaveField("UtsNS", ns))
	})
})
