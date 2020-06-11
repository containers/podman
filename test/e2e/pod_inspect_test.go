package integration

import (
	"os"

	. "github.com/containers/libpod/v2/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pod inspect", func() {
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

	It("podman inspect bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "inspect", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError())
	})

	It("podman inspect a pod", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"pod", "inspect", podid})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.IsJSONOutputValid()).To(BeTrue())
		podData := inspect.InspectPodToJSON()
		Expect(podData.ID).To(Equal(podid))
	})

	It("podman pod inspect (CreateCommand)", func() {
		Skip(v2remotefail)
		podName := "myTestPod"
		createCommand := []string{"pod", "create", "--name", podName, "--hostname", "rudolph", "--share", "net"}

		// Create the pod.
		session := podmanTest.Podman(createCommand)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Inspect the pod and make sure that the create command is
		// exactly how we created the pod.
		inspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.IsJSONOutputValid()).To(BeTrue())
		podData := inspect.InspectPodToJSON()
		// Let's get the last len(createCommand) items in the command.
		inspectCreateCommand := podData.CreateCommand
		index := len(inspectCreateCommand) - len(createCommand)
		Expect(inspectCreateCommand[index:]).To(Equal(createCommand))
	})
})
