// +build !remoteclient

package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/containers/libpod/test/utils"
	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman generate service kube on bogus object", func() {
		session := podmanTest.Podman([]string{"generate", "kube", "-s", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman generate kube on container", func() {
		session := podmanTest.RunTopContainer("top")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		_, err := yaml.Marshal(kube.OutputToString())
		Expect(err).To(BeNil())
	})

	It("podman generate service kube on container", func() {
		session := podmanTest.RunTopContainer("top")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		kube := podmanTest.Podman([]string{"generate", "kube", "-s", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		_, err := yaml.Marshal(kube.OutputToString())
		Expect(err).To(BeNil())
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

		_, err := yaml.Marshal(kube.OutputToString())
		Expect(err).To(BeNil())
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

		_, err := yaml.Marshal(kube.OutputToString())
		Expect(err).To(BeNil())
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

		kube := podmanTest.Podman([]string{"generate", "kube", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		err := ioutil.WriteFile(outputFile, []byte(kube.OutputToString()), 0644)
		Expect(err).To(BeNil())

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
})
