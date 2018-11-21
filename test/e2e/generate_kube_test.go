package integration

import (
	"fmt"
	"os"

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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))

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
})
