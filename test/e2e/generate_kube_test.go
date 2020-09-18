package integration

import (
	"os"
	"path/filepath"
	"strconv"

	. "github.com/containers/podman/v2/test/utils"
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
		_, rc, _ := podmanTest.CreatePod("toppod")
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

		numContainers := 0
		for range pod.Spec.Containers {
			numContainers = numContainers + 1
		}
		Expect(numContainers).To(Equal(1))
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
		_, rc, _ := podmanTest.CreatePod("toppod")
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
		_, rc, _ := podmanTest.CreatePod(podName)
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
		_, rc, _ := podmanTest.CreatePod(podName)
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
})
