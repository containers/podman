package integration

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/podman/v4/libpod/define"

	v1 "github.com/containers/podman/v4/pkg/k8s.io/api/core/v1"
	"github.com/containers/podman/v4/pkg/util"
	. "github.com/containers/podman/v4/test/utils"
	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman generate kube", func() {
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman generate pod kube on bogus object", func() {
		session := podmanTest.Podman([]string{"generate", "kube", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman generate service kube on bogus object", func() {
		session := podmanTest.Podman([]string{"generate", "kube", "-s", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman generate kube on container", func() {
		session := podmanTest.RunTopContainer("top")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"kube", "generate", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", false))
		Expect(pod.Spec.SecurityContext).To(BeNil())
		Expect(pod.Spec.DNSConfig).To(BeNil())
		Expect(pod.Spec.Containers[0]).To(HaveField("WorkingDir", ""))
		Expect(pod.Spec.Containers[0].SecurityContext).To(BeNil())
		Expect(pod.Spec.Containers[0].Env).To(BeNil())
		Expect(pod).To(HaveField("Name", "top-pod"))

		numContainers := 0
		for range pod.Spec.Containers {
			numContainers++
		}
		Expect(numContainers).To(Equal(1))
	})

	It("podman generate service kube on container with --security-opt level", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test", "--security-opt", "label=level:s0:c100,c200", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "test"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(kube.OutputToString()).To(ContainSubstring("level: s0:c100,c200"))
	})

	It("podman generate service kube on container with --security-opt disable", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test-disable", "--security-opt", "label=disable", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "test-disable"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(kube.OutputToString()).To(ContainSubstring("type: spc_t"))

	})

	It("podman generate service kube on container with --security-opt type", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test", "--security-opt", "label=type:foo_bar_t", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"kube", "generate", "test"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(kube.OutputToString()).To(ContainSubstring("type: foo_bar_t"))
	})

	It("podman generate service kube on container - targetPort should match port name", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test-ctr", "-p", "3890:3890", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"kube", "generate", "-s", "test-ctr"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Separate out the Service and Pod yaml
		arr := strings.Split(string(kube.Out.Contents()), "---")
		Expect(arr).To(HaveLen(2))

		svc := new(v1.Service)
		err := yaml.Unmarshal([]byte(arr[0]), svc)
		Expect(err).ToNot(HaveOccurred())
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].TargetPort.IntValue()).To(Equal(3890))

		pod := new(v1.Pod)
		err = yaml.Unmarshal([]byte(arr[1]), pod)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman generate kube on pod", func() {
		_, rc, _ := podmanTest.CreatePod(map[string][]string{"--name": {"toppod"}})
		Expect(rc).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("topcontainer", "toppod")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "toppod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", false))

		numContainers := 0
		for range pod.Spec.Containers {
			numContainers++
		}
		Expect(numContainers).To(Equal(1))
	})

	It("podman generate kube multiple pods", func() {
		pod1 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod1", ALPINE, "top"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1).Should(Exit(0))

		pod2 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod2", ALPINE, "top"})
		pod2.WaitWithDefaultTimeout()
		Expect(pod2).Should(Exit(0))

		kube := podmanTest.Podman([]string{"kube", "generate", "pod1", "pod2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		Expect(string(kube.Out.Contents())).To(ContainSubstring(`name: pod1`))
		Expect(string(kube.Out.Contents())).To(ContainSubstring(`name: pod2`))
	})

	It("podman generate kube on pod with init containers", func() {
		session := podmanTest.Podman([]string{"create", "--pod", "new:toppod", "--init-ctr", "always", ALPINE, "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "--pod", "toppod", "--init-ctr", "always", ALPINE, "echo", "world"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "--pod", "toppod", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"kube", "generate", "toppod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", false))

		numContainers := len(pod.Spec.Containers) + len(pod.Spec.InitContainers)
		Expect(numContainers).To(Equal(3))

		// Init container should be in the generated kube yaml if created with "once" type and the pod has not been started
		session = podmanTest.Podman([]string{"create", "--pod", "new:toppod-2", "--init-ctr", "once", ALPINE, "echo", "using once type"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "--pod", "toppod-2", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube = podmanTest.Podman([]string{"generate", "kube", "toppod-2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", false))

		numContainers = len(pod.Spec.Containers) + len(pod.Spec.InitContainers)
		Expect(numContainers).To(Equal(2))

		// Init container should not be in the generated kube yaml if created with "once" type and the pod has been started
		session = podmanTest.Podman([]string{"create", "--pod", "new:toppod-3", "--init-ctr", "once", ALPINE, "echo", "using once type"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "--pod", "toppod-3", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "start", "toppod-3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube = podmanTest.Podman([]string{"generate", "kube", "toppod-3"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", false))

		numContainers = len(pod.Spec.Containers) + len(pod.Spec.InitContainers)
		Expect(numContainers).To(Equal(1))
	})

	It("podman generate kube on pod with user namespace", func() {
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
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", "testPod", "--userns=auto"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(Exit(0))

		session := podmanTest.Podman([]string{"create", "--name", "topcontainer", "--pod", "testPod", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "testPod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		expected := false
		Expect(pod.Spec).To(HaveField("HostUsers", &expected))
	})

	It("podman generate kube on pod with host network", func() {
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", "testHostNetwork", "--network", "host"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(Exit(0))

		session := podmanTest.Podman([]string{"create", "--name", "topcontainer", "--pod", "testHostNetwork", "--network", "host", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "testHostNetwork"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", true))
	})

	It("podman generate kube on container with host network", func() {
		session := podmanTest.RunTopContainerWithArgs("topcontainer", []string{"--network", "host"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "topcontainer"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", true))
	})

	It("podman generate kube on pod with hostAliases", func() {
		podName := "testHost"
		testIP := "127.0.0.1"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName,
			"--add-host", "test1.podman.io" + ":" + testIP,
			"--add-host", "test2.podman.io" + ":" + testIP,
		})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(Exit(0))

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, ALPINE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session).Should(Exit(0))

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{"create", "--name", ctr2Name, "--pod", podName, ALPINE, "top"})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.HostAliases).To(HaveLen(2))
		Expect(pod.Spec.HostAliases[0]).To(HaveField("IP", testIP))
		Expect(pod.Spec.HostAliases[1]).To(HaveField("IP", testIP))
	})

	It("podman generate kube with network sharing", func() {
		// Expect error with default sharing options as Net namespace is shared
		podName := "testPod"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(Exit(0))

		ctrSession := podmanTest.Podman([]string{"create", "--name", "testCtr", "--pod", podName, "-p", "9000:8000", ALPINE, "top"})
		ctrSession.WaitWithDefaultTimeout()
		Expect(ctrSession).Should(Exit(125))

		// Ports without Net sharing should work with ports being set for each container in the generated kube yaml
		podName = "testNet"
		podSession = podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", "ipc"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(Exit(0))

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, "-p", "9000:8000", ALPINE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session).Should(Exit(0))

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{"create", "--name", ctr2Name, "--pod", podName, "-p", "6000:5000", ALPINE, "top"})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.Containers).To(HaveLen(2))
		Expect(pod.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(8000)))
		Expect(pod.Spec.Containers[1].Ports[0].ContainerPort).To(Equal(int32(5000)))
		Expect(pod.Spec.Containers[0].Ports[0].HostPort).To(Equal(int32(9000)))
		Expect(pod.Spec.Containers[1].Ports[0].HostPort).To(Equal(int32(6000)))
	})

	It("podman generate kube with and without hostname", func() {
		// Expect error with default sharing options as UTS namespace is shared
		podName := "testPod"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(Exit(0))

		ctrSession := podmanTest.Podman([]string{"create", "--name", "testCtr", "--pod", podName, "--hostname", "test-hostname", ALPINE, "top"})
		ctrSession.WaitWithDefaultTimeout()
		Expect(ctrSession).Should(Exit(125))

		// Hostname without uts sharing should work, but generated kube yaml will have pod hostname
		// set to the hostname of the first container
		podName = "testHostname"
		podSession = podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", "ipc"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(Exit(0))

		ctr1Name := "ctr1"
		ctr1HostName := "ctr1-hostname"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, "--hostname", ctr1HostName, ALPINE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session).Should(Exit(0))

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{"create", "--name", ctr2Name, "--pod", podName, ALPINE, "top"})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.Containers).To(HaveLen(2))
		Expect(pod.Spec.Hostname).To(Equal(ctr1HostName))

		// No hostname

		podName = "testNoHostname"
		podSession = podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", "ipc"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(Exit(0))

		ctr3Name := "ctr3"
		ctr3Session := podmanTest.Podman([]string{"create", "--name", ctr3Name, "--pod", podName, ALPINE, "top"})
		ctr3Session.WaitWithDefaultTimeout()
		Expect(ctr3Session).Should(Exit(0))

		kube = podmanTest.Podman([]string{"generate", "kube", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.Containers).To(HaveLen(1))
		Expect(pod.Spec.Hostname).To(BeEmpty())
	})

	It("podman generate service kube on pod", func() {
		session := podmanTest.Podman([]string{"create", "--pod", "new:test-pod", "-p", "4000:4000/udp", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "-s", "test-pod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Separate out the Service and Pod yaml
		arr := strings.Split(string(kube.Out.Contents()), "---")
		Expect(arr).To(HaveLen(2))

		svc := new(v1.Service)
		err := yaml.Unmarshal([]byte(arr[0]), svc)
		Expect(err).ToNot(HaveOccurred())
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].TargetPort.IntValue()).To(Equal(4000))
		Expect(svc.Spec.Ports[0]).To(HaveField("Protocol", v1.ProtocolUDP))

		pod := new(v1.Pod)
		err = yaml.Unmarshal([]byte(arr[1]), pod)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman generate kube on pod with restartPolicy", func() {
		// podName,  set,  expect
		testSli := [][]string{
			{"testPod1", "", "Never"}, // some pod create from cmdline, so set it to Never
			{"testPod2", "always", "Always"},
			{"testPod3", "on-failure", "OnFailure"},
			{"testPod4", "no", "Never"},
		}

		for k, v := range testSli {
			podName := v[0]
			podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName})
			podSession.WaitWithDefaultTimeout()
			Expect(podSession).Should(Exit(0))

			ctrName := "ctr" + strconv.Itoa(k)
			ctr1Session := podmanTest.Podman([]string{"create", "--name", ctrName, "--pod", podName,
				"--restart", v[1], ALPINE, "top"})
			ctr1Session.WaitWithDefaultTimeout()
			Expect(ctr1Session).Should(Exit(0))

			kube := podmanTest.Podman([]string{"generate", "kube", podName})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(Exit(0))

			pod := new(v1.Pod)
			err := yaml.Unmarshal(kube.Out.Contents(), pod)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(pod.Spec.RestartPolicy)).To(Equal(v[2]))
		}
	})

	It("podman generate kube on pod with memory limit", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		podName := "testMemoryLimit"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(Exit(0))

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, "--memory", "10M", ALPINE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		for _, ctr := range pod.Spec.Containers {
			memoryLimit, _ := ctr.Resources.Limits.Memory().AsInt64()
			Expect(memoryLimit).To(Equal(int64(10 * 1024 * 1024)))
		}
	})

	It("podman generate kube on pod with cpu limit", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		podName := "testCpuLimit"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(Exit(0))

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName,
			"--cpus", "0.5", ALPINE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session).Should(Exit(0))

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{"create", "--name", ctr2Name, "--pod", podName,
			"--cpu-period", "100000", "--cpu-quota", "50000", ALPINE, "top"})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		for _, ctr := range pod.Spec.Containers {
			cpuLimit := ctr.Resources.Limits.Cpu().MilliValue()
			Expect(cpuLimit).To(Equal(int64(500)))
		}
	})

	It("podman generate kube on pod with ports", func() {
		podName := "test"

		lock4 := GetPortLock("4000")
		defer lock4.Unlock()
		lock5 := GetPortLock("5000")
		defer lock5.Unlock()
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName, "-p", "4000:4000", "-p", "5000:5000"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(Exit(0))

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, ALPINE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session).Should(Exit(0))

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{"create", "--name", ctr2Name, "--pod", podName, ALPINE, "top"})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		foundPort4000 := 0
		foundPort5000 := 0
		foundOtherPort := 0
		for _, ctr := range pod.Spec.Containers {
			for _, port := range ctr.Ports {
				// Since we are using tcp here, the generated kube yaml shouldn't
				// have anything for protocol under the ports as tcp is the default
				// for k8s
				Expect(port.Protocol).To(BeEmpty())
				if port.HostPort == 4000 {
					foundPort4000++
				} else if port.HostPort == 5000 {
					foundPort5000++
				} else {
					foundOtherPort++
				}
			}
		}
		Expect(foundPort4000).To(Equal(1))
		Expect(foundPort5000).To(Equal(1))
		Expect(foundOtherPort).To(Equal(0))

		// Create container with UDP port and check the generated kube yaml
		ctrWithUDP := podmanTest.Podman([]string{"create", "--pod", "new:test-pod", "-p", "6666:66/udp", ALPINE, "top"})
		ctrWithUDP.WaitWithDefaultTimeout()
		Expect(ctrWithUDP).Should(Exit(0))

		kube = podmanTest.Podman([]string{"generate", "kube", "test-pod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers := pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0].Ports).To(HaveLen(1))
		Expect(containers[0].Ports[0]).To(HaveField("Protocol", v1.ProtocolUDP))
	})

	It("podman generate and reimport kube on pod", func() {
		podName := "toppod"
		_, rc, _ := podmanTest.CreatePod(map[string][]string{"--name": {podName}})
		Expect(rc).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", podName, "--name", "test1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session2 := podmanTest.Podman([]string{"create", "--pod", podName, "--name", "test2", ALPINE, "top"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"generate", "kube", "-f", outputFile, podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		session3 := podmanTest.Podman([]string{"pod", "rm", "-af"})
		session3.WaitWithDefaultTimeout()
		Expect(session3).Should(Exit(0))

		session4 := podmanTest.Podman([]string{"play", "kube", outputFile})
		session4.WaitWithDefaultTimeout()
		Expect(session4).Should(Exit(0))

		session5 := podmanTest.Podman([]string{"pod", "ps"})
		session5.WaitWithDefaultTimeout()
		Expect(session5).Should(Exit(0))
		Expect(session5.OutputToString()).To(ContainSubstring(podName))

		session6 := podmanTest.Podman([]string{"ps", "-a"})
		session6.WaitWithDefaultTimeout()
		Expect(session6).Should(Exit(0))
		psOut := session6.OutputToString()
		Expect(psOut).To(ContainSubstring("test1"))
		Expect(psOut).To(ContainSubstring("test2"))
	})

	It("podman generate with user and reimport kube on pod", func() {
		podName := "toppod"
		_, rc, _ := podmanTest.CreatePod(map[string][]string{"--name": {podName}})
		Expect(rc).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", podName, "--name", "test1", "--user", "100:200", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.Config.User}}", "test1"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("100:200"))

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"kube", "generate", "-f", outputFile, podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "rm", "-af"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		podmanTest.AddImageToRWStore(ALPINE)
		session = podmanTest.Podman([]string{"play", "kube", outputFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// container name in pod is <podName>-<ctrName>
		inspect1 := podmanTest.Podman([]string{"inspect", "--format", "{{.Config.User}}", "toppod-test1"})
		inspect1.WaitWithDefaultTimeout()
		Expect(inspect1).Should(Exit(0))
		Expect(inspect1.OutputToString()).To(ContainSubstring(inspect.OutputToString()))
	})

	It("podman generate kube with volume", func() {
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).ToNot(HaveOccurred())

		// we need a container name because IDs don't persist after rm/play
		ctrName := "test-ctr"
		ctrNameInKubePod := "test1-test-ctr"

		session1 := podmanTest.Podman([]string{"run", "-d", "--pod", "new:test1", "--name", ctrName, "-v", vol1 + ":/volume/:z", "alpine", "top"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"generate", "kube", "test1", "-f", outputFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		b, err := os.ReadFile(outputFile)
		Expect(err).ShouldNot(HaveOccurred())
		pod := new(v1.Pod)
		err = yaml.Unmarshal(b, pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.BindMountPrefix, vol1+":"+"z"))

		rm := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", "test1"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(Exit(0))

		play := podmanTest.Podman([]string{"play", "kube", outputFile})
		play.WaitWithDefaultTimeout()
		Expect(play).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrNameInKubePod})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(vol1))
	})

	It("podman generate kube when bind-mounting '/' and '/root' at the same time ", func() {
		// Fixes https://github.com/containers/podman/issues/9764

		ctrName := "mount-root-ctr"
		session1 := podmanTest.Podman([]string{"run", "-d", "--pod", "new:mount-root-conflict", "--name", ctrName,
			"-v", "/:/volume1/",
			"-v", "/root:/volume2/",
			"alpine", "top"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "mount-root-conflict"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.Volumes).To(HaveLen(2))

	})

	It("podman generate kube with persistent volume claim", func() {
		vol := "vol-test-persistent-volume-claim"

		// we need a container name because IDs don't persist after rm/play
		ctrName := "test-persistent-volume-claim"
		ctrNameInKubePod := "test1-test-persistent-volume-claim"

		session := podmanTest.Podman([]string{"run", "-d", "--pod", "new:test1", "--name", ctrName, "-v", vol + ":/volume/:z", "alpine", "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"generate", "kube", "test1", "-f", outputFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		rm := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", "test1"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(Exit(0))

		play := podmanTest.Podman([]string{"play", "kube", outputFile})
		play.WaitWithDefaultTimeout()
		Expect(play).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrNameInKubePod})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(vol))
	})

	It("podman generate kube sharing pid namespace", func() {
		podName := "test"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", "pid"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(Exit(0))

		session := podmanTest.Podman([]string{"create", "--pod", podName, "--name", "test1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"kube", "generate", podName, "-f", outputFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		rm := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", podName})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(Exit(0))

		play := podmanTest.Podman([]string{"play", "kube", outputFile})
		play.WaitWithDefaultTimeout()
		Expect(play).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`"pid"`))
	})

	It("podman generate kube with pods and containers", func() {
		pod1 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod1", ALPINE, "top"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1).Should(Exit(0))

		pod2 := podmanTest.Podman([]string{"run", "-dt", "--name", "top", ALPINE, "top"})
		pod2.WaitWithDefaultTimeout()
		Expect(pod2).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "pod1", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
	})

	It("podman generate kube with containers in a pod should fail", func() {
		pod1 := podmanTest.Podman([]string{"pod", "create", "--name", "pod1"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1).Should(Exit(0))

		con := podmanTest.Podman([]string{"run", "-dt", "--pod", "pod1", "--name", "top", ALPINE, "top"})
		con.WaitWithDefaultTimeout()
		Expect(con).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
	})

	It("podman generate kube with multiple containers", func() {
		con1 := podmanTest.Podman([]string{"run", "-dt", "--name", "con1", ALPINE, "top"})
		con1.WaitWithDefaultTimeout()
		Expect(con1).Should(Exit(0))

		con2 := podmanTest.Podman([]string{"run", "-dt", "--name", "con2", ALPINE, "top"})
		con2.WaitWithDefaultTimeout()
		Expect(con2).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "con1", "con2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
	})

	It("podman generate kube with containers in pods should fail", func() {
		pod1 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod1", "--name", "top1", ALPINE, "top"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1).Should(Exit(0))

		pod2 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod2", "--name", "top2", ALPINE, "top"})
		pod2.WaitWithDefaultTimeout()
		Expect(pod2).Should(Exit(0))

		kube := podmanTest.Podman([]string{"kube", "generate", "top1", "top2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
	})

	It("podman generate kube on a container with dns options", func() {
		top := podmanTest.Podman([]string{"run", "-dt", "--name", "top", "--dns", "8.8.8.8", "--dns-search", "foobar.com", "--dns-option", "color:blue", ALPINE, "top"})
		top.WaitWithDefaultTimeout()
		Expect(top).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.DNSConfig.Nameservers).To(ContainElement("8.8.8.8"))
		Expect(pod.Spec.DNSConfig.Searches).To(ContainElement("foobar.com"))
		Expect(pod.Spec.DNSConfig.Options).ToNot(BeEmpty())
		Expect(pod.Spec.DNSConfig.Options[0]).To(HaveField("Name", "color"))
		s := "blue"
		Expect(pod.Spec.DNSConfig.Options[0]).To(HaveField("Value", &s))
	})

	It("podman generate kube multiple container dns servers and options are cumulative", func() {
		top1 := podmanTest.Podman([]string{"run", "-dt", "--name", "top1", "--dns", "8.8.8.8", "--dns-search", "foobar.com", ALPINE, "top"})
		top1.WaitWithDefaultTimeout()
		Expect(top1).Should(Exit(0))

		top2 := podmanTest.Podman([]string{"run", "-dt", "--name", "top2", "--dns", "8.7.7.7", "--dns-search", "homer.com", ALPINE, "top"})
		top2.WaitWithDefaultTimeout()
		Expect(top2).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "top1", "top2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.DNSConfig.Nameservers).To(ContainElement("8.8.8.8"))
		Expect(pod.Spec.DNSConfig.Nameservers).To(ContainElement("8.7.7.7"))
		Expect(pod.Spec.DNSConfig.Searches).To(ContainElement("foobar.com"))
		Expect(pod.Spec.DNSConfig.Searches).To(ContainElement("homer.com"))
	})

	It("podman generate kube on a pod with dns options", func() {
		top := podmanTest.Podman([]string{"run", "--pod", "new:pod1", "-dt", "--name", "top", "--dns", "8.8.8.8", "--dns-search", "foobar.com", "--dns-opt", "color:blue", ALPINE, "top"})
		top.WaitWithDefaultTimeout()
		Expect(top).Should(Exit(0))

		kube := podmanTest.Podman([]string{"kube", "generate", "pod1"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.DNSConfig.Nameservers).To(ContainElement("8.8.8.8"))
		Expect(pod.Spec.DNSConfig.Searches).To(ContainElement("foobar.com"))
		Expect(pod.Spec.DNSConfig.Options).ToNot(BeEmpty())
		Expect(pod.Spec.DNSConfig.Options[0]).To(HaveField("Name", "color"))
		s := "blue"
		Expect(pod.Spec.DNSConfig.Options[0]).To(HaveField("Value", &s))
	})

	It("podman generate kube - set entrypoint as command", func() {
		session := podmanTest.Podman([]string{"create", "--pod", "new:testpod", "--entrypoint", "/bin/sleep", ALPINE, "10s"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Now make sure that the container's command is set to the
		// entrypoint and it's arguments to "10s".
		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers := pod.Spec.Containers
		Expect(containers).To(HaveLen(1))

		Expect(containers[0]).To(HaveField("Command", []string{"/bin/sleep"}))
		Expect(containers[0]).To(HaveField("Args", []string{"10s"}))
	})

	It("podman generate kube - use command from image unless explicitly set in the podman command", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "test"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Now make sure that the container's command in the kube yaml is not set to the
		// image command.
		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers := pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0].Command).To(BeEmpty())

		cmd := []string{"echo", "hi"}
		session = podmanTest.Podman(append([]string{"create", "--name", "test1", ALPINE}, cmd...))
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube = podmanTest.Podman([]string{"kube", "generate", "test1"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Now make sure that the container's command in the kube yaml is set to the
		// command passed via the cli to podman create.
		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers = pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0]).To(HaveField("Command", cmd))
	})

	It("podman generate kube - use entrypoint from image unless --entrypoint is set", func() {
		// Build an image with an entrypoint.
		containerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT ["sleep"]`

		targetPath, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())
		containerfilePath := filepath.Join(targetPath, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).ToNot(HaveOccurred())

		image := "generatekube:test"
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", containerfilePath, "-t", image})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "--pod", "new:testpod", image, "10s"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Now make sure that the container's command in the kube yaml is NOT set to the
		// entrypoint but the arguments should be set to "10s".
		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers := pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0]).To(HaveField("Args", []string{"10s"}))

		session = podmanTest.Podman([]string{"create", "--pod", "new:testpod-2", "--entrypoint", "echo", image, "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube = podmanTest.Podman([]string{"kube", "generate", "testpod-2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Now make sure that the container's command in the kube yaml is set to the
		// entrypoint defined by the --entrypoint flag and the arguments should be set to "hello".
		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers = pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0]).To(HaveField("Command", []string{"echo"}))
		Expect(containers[0]).To(HaveField("Args", []string{"hello"}))
	})

	It("podman generate kube - --privileged container", func() {
		session := podmanTest.Podman([]string{"create", "--pod", "new:testpod", "--privileged", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Now make sure that the capabilities aren't set.
		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers := pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0].SecurityContext.Capabilities).To(BeNil())

		// Now make sure we can also `play` it.
		kubeFile := filepath.Join(podmanTest.TempDir, "kube.yaml")

		kube = podmanTest.Podman([]string{"generate", "kube", "testpod", "-f", kubeFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Remove the pod so play can recreate it.
		kube = podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		kube = podmanTest.Podman([]string{"play", "kube", kubeFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
	})

	It("podman generate kube based on user in container", func() {
		// Build an image with an entrypoint.
		containerfile := `FROM quay.io/libpod/alpine:latest
RUN adduser -u 10001 -S test1
USER test1`

		targetPath, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())
		containerfilePath := filepath.Join(targetPath, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).ToNot(HaveOccurred())

		image := "generatekube:test"
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", containerfilePath, "-t", image})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "--pod", "new:testpod", image, "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.Containers[0].SecurityContext.RunAsUser).To(BeNil())
	})

	It("podman generate kube on named volume", func() {
		vol := "simple-named-volume"

		session := podmanTest.Podman([]string{"volume", "create", vol})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", vol})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pvc := new(v1.PersistentVolumeClaim)
		err := yaml.Unmarshal(kube.Out.Contents(), pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc).To(HaveField("Name", vol))
		Expect(pvc.Spec.AccessModes[0]).To(Equal(v1.ReadWriteOnce))
		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("1Gi"))
	})

	It("podman generate kube on named volume with options", func() {
		vol := "complex-named-volume"
		volDevice := "tmpfs"
		volType := "tmpfs"
		volOpts := "nodev,noexec"

		session := podmanTest.Podman([]string{"volume", "create", "--opt", "device=" + volDevice, "--opt", "type=" + volType, "--opt", "o=" + volOpts, vol})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", vol})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pvc := new(v1.PersistentVolumeClaim)
		err := yaml.Unmarshal(kube.Out.Contents(), pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc).To(HaveField("Name", vol))
		Expect(pvc.Spec.AccessModes[0]).To(Equal(v1.ReadWriteOnce))
		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("1Gi"))

		for k, v := range pvc.Annotations {
			switch k {
			case util.VolumeDeviceAnnotation:
				Expect(v).To(Equal(volDevice))
			case util.VolumeTypeAnnotation:
				Expect(v).To(Equal(volType))
			case util.VolumeMountOptsAnnotation:
				Expect(v).To(Equal(volOpts))
			}
		}
	})

	It("podman generate kube on container with auto update labels", func() {
		top := podmanTest.Podman([]string{"run", "-dt", "--name", "top", "--label", "io.containers.autoupdate=local", ALPINE, "top"})
		top.WaitWithDefaultTimeout()
		Expect(top).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Annotations).To(HaveKeyWithValue("io.containers.autoupdate/top", "local"))
	})

	It("podman generate kube on pod with auto update labels in all containers", func() {
		pod1 := podmanTest.Podman([]string{"pod", "create", "--name", "pod1"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1).Should(Exit(0))

		top1 := podmanTest.Podman([]string{"run", "-dt", "--name", "top1", "--pod", "pod1", "--label", "io.containers.autoupdate=registry", "--label", "io.containers.autoupdate.authfile=/some/authfile.json", ALPINE, "top"})
		top1.WaitWithDefaultTimeout()
		Expect(top1).Should(Exit(0))

		top2 := podmanTest.Podman([]string{"run", "-dt", "--name", "top2", "--workdir", "/root", "--pod", "pod1", "--label", "io.containers.autoupdate=registry", "--label", "io.containers.autoupdate.authfile=/some/authfile.json", ALPINE, "top"})
		top2.WaitWithDefaultTimeout()
		Expect(top2).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "pod1"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.Containers[0]).To(HaveField("WorkingDir", ""))
		Expect(pod.Spec.Containers[1]).To(HaveField("WorkingDir", "/root"))

		for _, ctr := range []string{"top1", "top2"} {
			Expect(pod.Annotations).To(HaveKeyWithValue("io.containers.autoupdate/"+ctr, "registry"))
			Expect(pod.Annotations).To(HaveKeyWithValue("io.containers.autoupdate.authfile/"+ctr, "/some/authfile.json"))
		}
	})

	It("podman generate kube can export env variables correctly", func() {
		// Fixes https://github.com/containers/podman/issues/12647
		// PR https://github.com/containers/podman/pull/12648

		ctrName := "gen-kube-env-ctr"
		podName := "gen-kube-env"
		// In proxy environment, this test needs to the --http-proxy=false option (#16684)
		session1 := podmanTest.Podman([]string{"run", "-d", "--http-proxy=false", "--pod", "new:" + podName, "--name", ctrName,
			"-e", "FOO=bar",
			"-e", "HELLO=WORLD",
			"alpine", "top"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.Containers[0].Env).To(HaveLen(2))
	})

	It("podman generate kube omit secret if empty", func() {
		dir, err := os.MkdirTemp(tempdir, "podman")
		Expect(err).ShouldNot(HaveOccurred())

		defer os.RemoveAll(dir)

		podCreate := podmanTest.Podman([]string{"run", "-d", "--pod", "new:" + "noSecretsPod", "--name", "noSecretsCtr", "--volume", dir + ":/foobar", ALPINE})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "noSecretsPod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		Expect(kube.OutputToString()).ShouldNot(ContainSubstring("secret"))

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.Volumes[0].Secret).To(BeNil())
	})
})
