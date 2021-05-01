package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/containers/podman/v3/pkg/util"
	. "github.com/containers/podman/v3/test/utils"
	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
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
		podmanTest.SeedImages()
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
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())
		Expect(pod.Spec.HostNetwork).To(Equal(false))

		numContainers := 0
		for range pod.Spec.Containers {
			numContainers = numContainers + 1
		}
		Expect(numContainers).To(Equal(1))
	})

	It("podman generate service kube on container with --security-opt level", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test", "--security-opt", "label=level:s0:c100,c200", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "test"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())
		Expect(kube.OutputToString()).To(ContainSubstring("level: s0:c100,c200"))
	})

	It("podman generate service kube on container with --security-opt disable", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test-disable", "--security-opt", "label=disable", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "test-disable"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())
		Expect(kube.OutputToString()).To(ContainSubstring("type: spc_t"))
	})

	It("podman generate service kube on container with --security-opt type", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test", "--security-opt", "label=type:foo_bar_t", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "test"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())
		Expect(kube.OutputToString()).To(ContainSubstring("type: foo_bar_t"))
	})

	It("podman generate service kube on container", func() {
		session := podmanTest.RunTopContainer("top")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "-s", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		// TODO - test generated YAML - service produces multiple
		// structs.
		// pod := new(v1.Pod)
		// err := yaml.Unmarshal([]byte(kube.OutputToString()), pod)
		// Expect(err).To(BeNil())
	})

	It("podman generate kube on pod", func() {
		_, rc, _ := podmanTest.CreatePod(map[string][]string{"--name": {"toppod"}})
		Expect(rc).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("topcontainer", "toppod")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "toppod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())
		Expect(pod.Spec.HostNetwork).To(Equal(false))

		numContainers := 0
		for range pod.Spec.Containers {
			numContainers = numContainers + 1
		}
		Expect(numContainers).To(Equal(1))
	})

	It("podman generate kube multiple pods", func() {
		pod1 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod1", ALPINE, "top"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1.ExitCode()).To(Equal(0))

		pod2 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod2", ALPINE, "top"})
		pod2.WaitWithDefaultTimeout()
		Expect(pod2.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "pod1", "pod2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		Expect(string(kube.Out.Contents())).To(ContainSubstring(`name: pod1`))
		Expect(string(kube.Out.Contents())).To(ContainSubstring(`name: pod2`))
	})

	It("podman generate kube on pod with host network", func() {
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", "testHostNetwork", "--network", "host"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--name", "topcontainer", "--pod", "testHostNetwork", "--network", "host", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "testHostNetwork"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())
		Expect(pod.Spec.HostNetwork).To(Equal(true))
	})

	It("podman generate kube on container with host network", func() {
		session := podmanTest.RunTopContainerWithArgs("topcontainer", []string{"--network", "host"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "topcontainer"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())
		Expect(pod.Spec.HostNetwork).To(Equal(true))
	})

	It("podman generate kube on pod with hostAliases", func() {
		podName := "testHost"
		testIP := "127.0.0.1"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName,
			"--add-host", "test1.podman.io" + ":" + testIP,
			"--add-host", "test2.podman.io" + ":" + testIP,
		})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession.ExitCode()).To(Equal(0))

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, ALPINE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session.ExitCode()).To(Equal(0))

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{"create", "--name", ctr2Name, "--pod", podName, ALPINE, "top"})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())
		Expect(len(pod.Spec.HostAliases)).To(Equal(2))
		Expect(pod.Spec.HostAliases[0].IP).To(Equal(testIP))
		Expect(pod.Spec.HostAliases[1].IP).To(Equal(testIP))
	})

	It("podman generate service kube on pod", func() {
		_, rc, _ := podmanTest.CreatePod(map[string][]string{"--name": {"toppod"}})
		Expect(rc).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("topcontainer", "toppod")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "-s", "toppod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		// TODO: How do we test unmarshal with a service? We have two
		// structs that need to be unmarshalled...
		// _, err := yaml.Marshal(kube.OutputToString())
		// Expect(err).To(BeNil())
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
			Expect(podSession.ExitCode()).To(Equal(0))

			ctrName := "ctr" + strconv.Itoa(k)
			ctr1Session := podmanTest.Podman([]string{"create", "--name", ctrName, "--pod", podName,
				"--restart", v[1], ALPINE, "top"})
			ctr1Session.WaitWithDefaultTimeout()
			Expect(ctr1Session.ExitCode()).To(Equal(0))

			kube := podmanTest.Podman([]string{"generate", "kube", podName})
			kube.WaitWithDefaultTimeout()
			Expect(kube.ExitCode()).To(Equal(0))

			pod := new(v1.Pod)
			err := yaml.Unmarshal(kube.Out.Contents(), pod)
			Expect(err).To(BeNil())

			Expect(string(pod.Spec.RestartPolicy)).To(Equal(v[2]))
		}
	})

	It("podman generate kube on pod with memory limit", func() {
		podName := "testMemoryLimit"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession.ExitCode()).To(Equal(0))

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, "--memory", "10Mi", ALPINE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())

		for _, ctr := range pod.Spec.Containers {
			memoryLimit, _ := ctr.Resources.Limits.Memory().AsInt64()
			Expect(memoryLimit).To(Equal(int64(10 * 1024 * 1024)))
		}
	})

	It("podman generate kube on pod with cpu limit", func() {
		podName := "testCpuLimit"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession.ExitCode()).To(Equal(0))

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName,
			"--cpus", "0.5", ALPINE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session.ExitCode()).To(Equal(0))

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{"create", "--name", ctr2Name, "--pod", podName,
			"--cpu-period", "100000", "--cpu-quota", "50000", ALPINE, "top"})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())

		for _, ctr := range pod.Spec.Containers {
			cpuLimit := ctr.Resources.Limits.Cpu().MilliValue()
			Expect(cpuLimit).To(Equal(int64(500)))
		}
	})

	It("podman generate kube on pod with ports", func() {
		podName := "test"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName, "-p", "4000:4000", "-p", "5000:5000"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession.ExitCode()).To(Equal(0))

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, ALPINE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session.ExitCode()).To(Equal(0))

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{"create", "--name", ctr2Name, "--pod", podName, ALPINE, "top"})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())

		foundPort4000 := 0
		foundPort5000 := 0
		foundOtherPort := 0
		for _, ctr := range pod.Spec.Containers {
			for _, port := range ctr.Ports {
				if port.HostPort == 4000 {
					foundPort4000 = foundPort4000 + 1
				} else if port.HostPort == 5000 {
					foundPort5000 = foundPort5000 + 1
				} else {
					foundOtherPort = foundOtherPort + 1
				}
			}
		}
		Expect(foundPort4000).To(Equal(1))
		Expect(foundPort5000).To(Equal(1))
		Expect(foundOtherPort).To(Equal(0))
	})

	It("podman generate and reimport kube on pod", func() {
		podName := "toppod"
		_, rc, _ := podmanTest.CreatePod(map[string][]string{"--name": {podName}})
		Expect(rc).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", podName, "--name", "test1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"create", "--pod", podName, "--name", "test2", ALPINE, "top"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"generate", "kube", "-f", outputFile, podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		session3 := podmanTest.Podman([]string{"pod", "rm", "-af"})
		session3.WaitWithDefaultTimeout()
		Expect(session3.ExitCode()).To(Equal(0))

		session4 := podmanTest.Podman([]string{"play", "kube", outputFile})
		session4.WaitWithDefaultTimeout()
		Expect(session4.ExitCode()).To(Equal(0))

		session5 := podmanTest.Podman([]string{"pod", "ps"})
		session5.WaitWithDefaultTimeout()
		Expect(session5.ExitCode()).To(Equal(0))
		Expect(session5.OutputToString()).To(ContainSubstring(podName))

		session6 := podmanTest.Podman([]string{"ps", "-a"})
		session6.WaitWithDefaultTimeout()
		Expect(session6.ExitCode()).To(Equal(0))
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
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.Config.User}}", "test1"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("100:200"))

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"generate", "kube", "-f", outputFile, podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "rm", "-af"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		podmanTest.AddImageToRWStore(ALPINE)
		session = podmanTest.Podman([]string{"play", "kube", outputFile})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// container name in pod is <podName>-<ctrName>
		inspect1 := podmanTest.Podman([]string{"inspect", "--format", "{{.Config.User}}", "toppod-test1"})
		inspect1.WaitWithDefaultTimeout()
		Expect(inspect1.ExitCode()).To(Equal(0))
		Expect(inspect1.OutputToString()).To(ContainSubstring(inspect.OutputToString()))
	})

	It("podman generate kube with volume", func() {
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).To(BeNil())

		// we need a container name because IDs don't persist after rm/play
		ctrName := "test-ctr"
		ctrNameInKubePod := "test1-test-ctr"

		session1 := podmanTest.Podman([]string{"run", "-d", "--pod", "new:test1", "--name", ctrName, "-v", vol1 + ":/volume/:z", "alpine", "top"})
		session1.WaitWithDefaultTimeout()
		Expect(session1.ExitCode()).To(Equal(0))

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"generate", "kube", "test1", "-f", outputFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		rm := podmanTest.Podman([]string{"pod", "rm", "-f", "test1"})
		rm.WaitWithDefaultTimeout()
		Expect(rm.ExitCode()).To(Equal(0))

		play := podmanTest.Podman([]string{"play", "kube", outputFile})
		play.WaitWithDefaultTimeout()
		Expect(play.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrNameInKubePod})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
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
		Expect(session1.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "mount-root-conflict"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())

		Expect(len(pod.Spec.Volumes)).To(Equal(2))

	})

	It("podman generate kube with persistent volume claim", func() {
		vol := "vol-test-persistent-volume-claim"

		// we need a container name because IDs don't persist after rm/play
		ctrName := "test-persistent-volume-claim"
		ctrNameInKubePod := "test1-test-persistent-volume-claim"

		session := podmanTest.Podman([]string{"run", "-d", "--pod", "new:test1", "--name", ctrName, "-v", vol + ":/volume/:z", "alpine", "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"generate", "kube", "test1", "-f", outputFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		rm := podmanTest.Podman([]string{"pod", "rm", "-f", "test1"})
		rm.WaitWithDefaultTimeout()
		Expect(rm.ExitCode()).To(Equal(0))

		play := podmanTest.Podman([]string{"play", "kube", outputFile})
		play.WaitWithDefaultTimeout()
		Expect(play.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrNameInKubePod})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(vol))
	})

	It("podman generate kube sharing pid namespace", func() {
		podName := "test"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", "pid"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", podName, "--name", "test1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"generate", "kube", podName, "-f", outputFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		rm := podmanTest.Podman([]string{"pod", "rm", "-f", podName})
		rm.WaitWithDefaultTimeout()
		Expect(rm.ExitCode()).To(Equal(0))

		play := podmanTest.Podman([]string{"play", "kube", outputFile})
		play.WaitWithDefaultTimeout()
		Expect(play.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`"pid"`))
	})

	It("podman generate kube with pods and containers", func() {
		pod1 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod1", ALPINE, "top"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1.ExitCode()).To(Equal(0))

		pod2 := podmanTest.Podman([]string{"run", "-dt", "--name", "top", ALPINE, "top"})
		pod2.WaitWithDefaultTimeout()
		Expect(pod2.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "pod1", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))
	})

	It("podman generate kube with containers in a pod should fail", func() {
		pod1 := podmanTest.Podman([]string{"pod", "create", "--name", "pod1"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1.ExitCode()).To(Equal(0))

		con := podmanTest.Podman([]string{"run", "-dt", "--pod", "pod1", "--name", "top", ALPINE, "top"})
		con.WaitWithDefaultTimeout()
		Expect(con.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).ToNot(Equal(0))
	})

	It("podman generate kube with multiple containers", func() {
		con1 := podmanTest.Podman([]string{"run", "-dt", "--name", "con1", ALPINE, "top"})
		con1.WaitWithDefaultTimeout()
		Expect(con1.ExitCode()).To(Equal(0))

		con2 := podmanTest.Podman([]string{"run", "-dt", "--name", "con2", ALPINE, "top"})
		con2.WaitWithDefaultTimeout()
		Expect(con2.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "con1", "con2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))
	})

	It("podman generate kube with containers in pods should fail", func() {
		pod1 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod1", "--name", "top1", ALPINE, "top"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1.ExitCode()).To(Equal(0))

		pod2 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod2", "--name", "top2", ALPINE, "top"})
		pod2.WaitWithDefaultTimeout()
		Expect(pod2.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "top1", "top2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).ToNot(Equal(0))
	})

	It("podman generate kube on a container with dns options", func() {
		top := podmanTest.Podman([]string{"run", "-dt", "--name", "top", "--dns", "8.8.8.8", "--dns-search", "foobar.com", "--dns-opt", "color:blue", ALPINE, "top"})
		top.WaitWithDefaultTimeout()
		Expect(top.ExitCode()).To(BeZero())

		kube := podmanTest.Podman([]string{"generate", "kube", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())

		Expect(StringInSlice("8.8.8.8", pod.Spec.DNSConfig.Nameservers)).To(BeTrue())
		Expect(StringInSlice("foobar.com", pod.Spec.DNSConfig.Searches)).To(BeTrue())
		Expect(len(pod.Spec.DNSConfig.Options)).To(BeNumerically(">", 0))
		Expect(pod.Spec.DNSConfig.Options[0].Name).To(Equal("color"))
		Expect(*pod.Spec.DNSConfig.Options[0].Value).To(Equal("blue"))
	})

	It("podman generate kube multiple container dns servers and options are cumulative", func() {
		top1 := podmanTest.Podman([]string{"run", "-dt", "--name", "top1", "--dns", "8.8.8.8", "--dns-search", "foobar.com", ALPINE, "top"})
		top1.WaitWithDefaultTimeout()
		Expect(top1.ExitCode()).To(BeZero())

		top2 := podmanTest.Podman([]string{"run", "-dt", "--name", "top2", "--dns", "8.7.7.7", "--dns-search", "homer.com", ALPINE, "top"})
		top2.WaitWithDefaultTimeout()
		Expect(top2.ExitCode()).To(BeZero())

		kube := podmanTest.Podman([]string{"generate", "kube", "top1", "top2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())

		Expect(StringInSlice("8.8.8.8", pod.Spec.DNSConfig.Nameservers)).To(BeTrue())
		Expect(StringInSlice("8.7.7.7", pod.Spec.DNSConfig.Nameservers)).To(BeTrue())
		Expect(StringInSlice("foobar.com", pod.Spec.DNSConfig.Searches)).To(BeTrue())
		Expect(StringInSlice("homer.com", pod.Spec.DNSConfig.Searches)).To(BeTrue())
	})

	It("podman generate kube on a pod with dns options", func() {
		top := podmanTest.Podman([]string{"run", "--pod", "new:pod1", "-dt", "--name", "top", "--dns", "8.8.8.8", "--dns-search", "foobar.com", "--dns-opt", "color:blue", ALPINE, "top"})
		top.WaitWithDefaultTimeout()
		Expect(top.ExitCode()).To(BeZero())

		kube := podmanTest.Podman([]string{"generate", "kube", "pod1"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())

		Expect(StringInSlice("8.8.8.8", pod.Spec.DNSConfig.Nameservers)).To(BeTrue())
		Expect(StringInSlice("foobar.com", pod.Spec.DNSConfig.Searches)).To(BeTrue())
		Expect(len(pod.Spec.DNSConfig.Options)).To(BeNumerically(">", 0))
		Expect(pod.Spec.DNSConfig.Options[0].Name).To(Equal("color"))
		Expect(*pod.Spec.DNSConfig.Options[0].Value).To(Equal("blue"))
	})

	It("podman generate kube - set entrypoint as command", func() {
		session := podmanTest.Podman([]string{"create", "--pod", "new:testpod", "--entrypoint", "/bin/sleep", ALPINE, "10s"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		// Now make sure that the container's command is set to the
		// entrypoint and it's arguments to "10s".
		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())

		containers := pod.Spec.Containers
		Expect(len(containers)).To(Equal(1))

		Expect(containers[0].Command).To(Equal([]string{"/bin/sleep"}))
		Expect(containers[0].Args).To(Equal([]string{"10s"}))
	})

	It("podman generate kube - use entrypoint from image", func() {
		// Build an image with an entrypoint.
		containerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT /bin/sleep`

		targetPath, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())
		containerfilePath := filepath.Join(targetPath, "Containerfile")
		err = ioutil.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).To(BeNil())

		image := "generatekube:test"
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", containerfilePath, "-t", image})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", "new:testpod", image, "10s"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		// Now make sure that the container's command is set to the
		// entrypoint and it's arguments to "10s".
		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())

		containers := pod.Spec.Containers
		Expect(len(containers)).To(Equal(1))

		Expect(containers[0].Command).To(Equal([]string{"/bin/sh", "-c", "/bin/sleep"}))
		Expect(containers[0].Args).To(Equal([]string{"10s"}))
	})

	It("podman generate kube - --privileged container", func() {
		session := podmanTest.Podman([]string{"create", "--pod", "new:testpod", "--privileged", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		// Now make sure that the capabilities aren't set.
		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())

		containers := pod.Spec.Containers
		Expect(len(containers)).To(Equal(1))
		Expect(containers[0].SecurityContext.Capabilities).To(BeNil())

		// Now make sure we can also `play` it.
		kubeFile := filepath.Join(podmanTest.TempDir, "kube.yaml")

		kube = podmanTest.Podman([]string{"generate", "kube", "testpod", "-f", kubeFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		// Remove the pod so play can recreate it.
		kube = podmanTest.Podman([]string{"pod", "rm", "-f", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		kube = podmanTest.Podman([]string{"play", "kube", kubeFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))
	})

	It("podman generate kube based on user in container", func() {
		// Build an image with an entrypoint.
		containerfile := `FROM quay.io/libpod/alpine:latest
RUN adduser -u 10001 -S test1
USER test1`

		targetPath, err := CreateTempDirInTempDir()
		Expect(err).To(BeNil())
		containerfilePath := filepath.Join(targetPath, "Containerfile")
		err = ioutil.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).To(BeNil())

		image := "generatekube:test"
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", containerfilePath, "-t", image})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", "new:testpod", image, "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())
		Expect(*pod.Spec.Containers[0].SecurityContext.RunAsUser).To(Equal(int64(10001)))
	})

	It("podman generate kube on named volume", func() {
		vol := "simple-named-volume"

		session := podmanTest.Podman([]string{"volume", "create", vol})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", vol})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pvc := new(v1.PersistentVolumeClaim)
		err := yaml.Unmarshal(kube.Out.Contents(), pvc)
		Expect(err).To(BeNil())
		Expect(pvc.GetName()).To(Equal(vol))
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
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", vol})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pvc := new(v1.PersistentVolumeClaim)
		err := yaml.Unmarshal(kube.Out.Contents(), pvc)
		Expect(err).To(BeNil())
		Expect(pvc.GetName()).To(Equal(vol))
		Expect(pvc.Spec.AccessModes[0]).To(Equal(v1.ReadWriteOnce))
		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("1Gi"))

		for k, v := range pvc.GetAnnotations() {
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
		Expect(top.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())

		v, ok := pod.GetAnnotations()["io.containers.autoupdate/top"]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal("local"))
	})

	It("podman generate kube on pod with auto update labels in all containers", func() {
		pod1 := podmanTest.Podman([]string{"pod", "create", "--name", "pod1"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1.ExitCode()).To(Equal(0))

		top1 := podmanTest.Podman([]string{"run", "-dt", "--name", "top1", "--pod", "pod1", "--label", "io.containers.autoupdate=registry", "--label", "io.containers.autoupdate.authfile=/some/authfile.json", ALPINE, "top"})
		top1.WaitWithDefaultTimeout()
		Expect(top1.ExitCode()).To(Equal(0))

		top2 := podmanTest.Podman([]string{"run", "-dt", "--name", "top2", "--pod", "pod1", "--label", "io.containers.autoupdate=registry", "--label", "io.containers.autoupdate.authfile=/some/authfile.json", ALPINE, "top"})
		top2.WaitWithDefaultTimeout()
		Expect(top2.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "pod1"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).To(BeNil())

		for _, ctr := range []string{"top1", "top2"} {
			v, ok := pod.GetAnnotations()["io.containers.autoupdate/"+ctr]
			Expect(ok).To(Equal(true))
			Expect(v).To(Equal("registry"))

			v, ok = pod.GetAnnotations()["io.containers.autoupdate.authfile/"+ctr]
			Expect(ok).To(Equal(true))
			Expect(v).To(Equal("/some/authfile.json"))
		}
	})
})
