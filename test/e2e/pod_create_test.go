package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/podman/v3/pkg/rootless"
	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman pod create", func() {
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

	It("podman create pod", func() {
		_, ec, podID := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		match, _ := check.GrepString(podID)
		Expect(match).To(BeTrue())
		Expect(len(check.OutputToStringArray())).To(Equal(1))
	})

	It("podman create pod with name", func() {
		name := "test"
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {name}})
		Expect(ec).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		match, _ := check.GrepString(name)
		Expect(match).To(BeTrue())
	})

	It("podman create pod with doubled name", func() {
		name := "test"
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {name}})
		Expect(ec).To(Equal(0))

		_, ec2, _ := podmanTest.CreatePod(map[string][]string{"--name": {name}})
		Expect(ec2).To(Not(Equal(0)))

		check := podmanTest.Podman([]string{"pod", "ps", "-q"})
		check.WaitWithDefaultTimeout()
		Expect(len(check.OutputToStringArray())).To(Equal(1))
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
		Expect(len(check.OutputToStringArray())).To(Equal(0))
	})

	It("podman create pod without network portbindings", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		pod := session.OutputToString()

		webserver := podmanTest.Podman([]string{"run", "--pod", pod, "-dt", nginx})
		webserver.WaitWithDefaultTimeout()
		Expect(webserver).Should(Exit(0))

		check := SystemExec("nc", []string{"-z", "localhost", "80"})
		Expect(check).Should(Exit(1))
	})

	It("podman create pod with network portbindings", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--name", name, "-p", "8080:80"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		pod := session.OutputToString()

		webserver := podmanTest.Podman([]string{"run", "--pod", pod, "-dt", nginx})
		webserver.WaitWithDefaultTimeout()
		Expect(webserver).Should(Exit(0))

		check := SystemExec("nc", []string{"-z", "localhost", "8080"})
		Expect(check).Should(Exit(0))
	})

	It("podman create pod with id file with network portbindings", func() {
		file := filepath.Join(podmanTest.TempDir, "pod.id")
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--name", name, "--pod-id-file", file, "-p", "8080:80"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		webserver := podmanTest.Podman([]string{"run", "--pod-id-file", file, "-dt", nginx})
		webserver.WaitWithDefaultTimeout()
		Expect(webserver).Should(Exit(0))

		check := SystemExec("nc", []string{"-z", "localhost", "8080"})
		Expect(check).Should(Exit(0))
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
		Expect(strings.Contains(podResolvConf.OutputToString(), "12.34.56.78 test.example.com")).To(BeTrue())
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
		Expect(strings.Contains(podResolvConf.OutputToString(), fmt.Sprintf("nameserver %s", server))).To(BeTrue())
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
		Expect(strings.Contains(podResolvConf.OutputToString(), fmt.Sprintf("options %s", option))).To(BeTrue())
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
		Expect(strings.Contains(podResolvConf.OutputToString(), fmt.Sprintf("search %s", search))).To(BeTrue())
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
		if rootless.IsRootless() {
			Expect(podCreate).Should(Exit(125))
		} else {
			Expect(podCreate).Should(Exit(0))
			podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "-ti", "--rm", ALPINE, "ip", "addr"})
			podResolvConf.WaitWithDefaultTimeout()
			Expect(podResolvConf).Should(Exit(0))
			Expect(strings.Contains(podResolvConf.OutputToString(), ip)).To(BeTrue())
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
		Expect(ctrJSON[0].NetworkSettings.IPAddress).To(Equal(ip))
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
		if rootless.IsRootless() {
			Expect(podCreate).Should(Exit(125))
		} else {
			Expect(podCreate).Should(Exit(0))
			podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "-ti", "--rm", ALPINE, "ip", "addr"})
			podResolvConf.WaitWithDefaultTimeout()
			Expect(podResolvConf).Should(Exit(0))
			Expect(strings.Contains(podResolvConf.OutputToString(), mac)).To(BeTrue())
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
		Expect(err).To(BeNil())
		Expect(os.Chdir(os.TempDir())).To(BeNil())
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

		id, _ := ioutil.ReadFile(targetFile)
		check := podmanTest.Podman([]string{"pod", "inspect", "abc"})
		check.WaitWithDefaultTimeout()
		data := check.InspectPodToJSON()
		Expect(data.ID).To(Equal(string(id)))
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
		Expect(check1.OutputToString()).To(Equal("/pause"))

		// check the Path and Args
		check2 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Path}}:{{.Args}}", data.Containers[0].ID})
		check2.WaitWithDefaultTimeout()
		Expect(check2).Should(Exit(0))
		Expect(check2.OutputToString()).To(Equal("/pause:[/pause]"))
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
		Expect(strings.Contains(status1.OutputToString(), "Created")).To(BeTrue())

		ctr1 := podmanTest.Podman([]string{"run", "--pod", podName, "-d", ALPINE, "top"})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(Exit(0))

		status2 := podmanTest.Podman([]string{"pod", "inspect", "--format", "{{ .State }}", podName})
		status2.WaitWithDefaultTimeout()
		Expect(status2).Should(Exit(0))
		Expect(strings.Contains(status2.OutputToString(), "Running")).To(BeTrue())

		ctr2 := podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(Exit(0))

		status3 := podmanTest.Podman([]string{"pod", "inspect", "--format", "{{ .State }}", podName})
		status3.WaitWithDefaultTimeout()
		Expect(status3).Should(Exit(0))
		Expect(strings.Contains(status3.OutputToString(), "Degraded")).To(BeTrue())
	})

	It("podman create with unsupported network options", func() {
		podCreate := podmanTest.Podman([]string{"pod", "create", "--network", "container:doesnotmatter"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(125))
		Expect(podCreate.ErrorToString()).To(ContainSubstring("pods presently do not support network mode container"))

		podCreate = podmanTest.Podman([]string{"pod", "create", "--network", "ns:/does/not/matter"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(125))
		Expect(podCreate.ErrorToString()).To(ContainSubstring("pods presently do not support network mode path"))
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
		Expect(len(session.OutputToStringArray())).To(Equal(1))
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
		Expect(podJSON.InfraConfig.PidNS).To(Equal(ns))

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
		Expect(podJSON.InfraConfig.PidNS).To(Equal("host"))

		podName = "pidPod4"
		ns = "private"

		podCreate = podmanTest.Podman([]string{"pod", "create", "--pid", ns, "--name", podName, "--share", "pid"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podInspect = podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(Exit(0))
		podJSON = podInspect.InspectPodToJSON()
		Expect(podJSON.InfraConfig.PidNS).To(Equal("private"))

		podName = "pidPod5"
		ns = "container:randomfakeid"

		podCreate = podmanTest.Podman([]string{"pod", "create", "--pid", ns, "--name", podName, "--share", "pid"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError())

	})

	It("podman pod create with --userns=keep-id", func() {
		if os.Geteuid() == 0 {
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
		ok, _ := session.GrepString(uid)
		Expect(ok).To(BeTrue())

		// Check passwd
		session = podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "id", "-un"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		u, err := user.Current()
		Expect(err).To(BeNil())
		ok, _ = session.GrepString(u.Name)
		Expect(ok).To(BeTrue())

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
		if os.Geteuid() == 0 {
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

		// container inside pod inherits user form infra container if --user is not set
		// etc/passwd entry will look like 1000:*:1000:1000:container user:/:/bin/sh
		exec1 := podmanTest.Podman([]string{"exec", ctrName, "cat", "/etc/passwd"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(Exit(0))
		Expect(exec1.OutputToString()).To(ContainSubstring("container"))

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
		Expect(err).To(BeNil())
		name := u.Name
		if name == "root" {
			name = "containers"
		}

		content, err := ioutil.ReadFile("/etc/subuid")
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
			Expect(strings.Contains(l, "1024")).To(BeTrue())
			m[l] = l
		}
		// check for no duplicates
		Expect(len(m)).To(Equal(5))
	})

	It("podman pod create --userns=auto:size=%d", func() {
		u, err := user.Current()
		Expect(err).To(BeNil())

		name := u.Name
		if name == "root" {
			name = "containers"
		}

		content, err := ioutil.ReadFile("/etc/subuid")
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
		ok, _ := session.GrepString("500")

		podName = "testPod-1"
		podCreate = podmanTest.Podman([]string{"pod", "create", "--userns=auto:size=3000", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))
		session = podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		ok, _ = session.GrepString("3000")

		Expect(ok).To(BeTrue())
	})

	It("podman pod create --userns=auto:uidmapping=", func() {
		u, err := user.Current()
		Expect(err).To(BeNil())

		name := u.Name
		if name == "root" {
			name = "containers"
		}

		content, err := ioutil.ReadFile("/etc/subuid")
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
		ok, _ := session.GrepString("8191")
		Expect(ok).To(BeTrue())
	})

	It("podman pod create --userns=auto:gidmapping=", func() {
		u, err := user.Current()
		Expect(err).To(BeNil())

		name := u.Name
		if name == "root" {
			name = "containers"
		}

		content, err := ioutil.ReadFile("/etc/subuid")
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
		ok, _ := session.GrepString("8191")
		Expect(ok).To(BeTrue())
	})

})
