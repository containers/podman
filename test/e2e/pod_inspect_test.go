package integration

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/podman/v3/libpod/define"
	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"pod", "inspect", podid})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.IsJSONOutputValid()).To(BeTrue())
		podData := inspect.InspectPodToJSON()
		Expect(podData.ID).To(Equal(podid))
	})

	It("podman pod inspect (CreateCommand)", func() {
		podName := "myTestPod"
		createCommand := []string{"pod", "create", "--name", podName, "--hostname", "rudolph", "--share", "net"}

		// Create the pod.
		session := podmanTest.Podman(createCommand)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Inspect the pod and make sure that the create command is
		// exactly how we created the pod.
		inspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.IsJSONOutputValid()).To(BeTrue())
		podData := inspect.InspectPodToJSON()
		// Let's get the last len(createCommand) items in the command.
		inspectCreateCommand := podData.CreateCommand
		index := len(inspectCreateCommand) - len(createCommand)
		Expect(inspectCreateCommand[index:]).To(Equal(createCommand))
	})

	It("podman pod inspect outputs port bindings", func() {
		podName := "testPod"
		create := podmanTest.Podman([]string{"pod", "create", "--name", podName, "-p", "8383:80"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		inspectOut := podmanTest.Podman([]string{"pod", "inspect", podName})
		inspectOut.WaitWithDefaultTimeout()
		Expect(inspectOut).Should(Exit(0))

		inspectJSON := new(define.InspectPodData)
		err := json.Unmarshal(inspectOut.Out.Contents(), inspectJSON)
		Expect(err).To(BeNil())
		Expect(inspectJSON.InfraConfig).To(Not(BeNil()))
		Expect(len(inspectJSON.InfraConfig.PortBindings["80/tcp"])).To(Equal(1))
		Expect(inspectJSON.InfraConfig.PortBindings["80/tcp"][0].HostPort).To(Equal("8383"))
	})

	It("podman pod inspect outputs show correct MAC", func() {
		SkipIfRootless("--mac-address is not supported in rootless mode without network")
		podName := "testPod"
		macAddr := "42:43:44:00:00:01"
		create := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--mac-address", macAddr})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		create = podmanTest.Podman([]string{"run", "-d", "--pod", podName, ALPINE, "top"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		inspectOut := podmanTest.Podman([]string{"pod", "inspect", podName})
		inspectOut.WaitWithDefaultTimeout()
		Expect(inspectOut).Should(Exit(0))

		Expect(inspectOut.OutputToString()).To(ContainSubstring(macAddr))
	})

	for _, ns := range []string{"pid", "ipc", "net", "uts", "cgroup", ""} {
		// This is important to move the 'log' var to the correct scope under Ginkgo flow.
		ns := ns
		testName := fmt.Sprintf("podman pod inspect outputs correct shared namespaces (%s)", ns)

		It(testName, func() {
			podName := "testPod"
			create := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", ns})
			create.WaitWithDefaultTimeout()
			Expect(create).Should(Exit(0))

			inspect := podmanTest.Podman([]string{"pod", "inspect", podName})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(Exit(0))
			Expect(inspect.IsJSONOutputValid()).To(BeTrue())
			if ns != "" {
				Expect(inspect.InspectPodToJSON().SharedNamespaces).To(Equal([]string{ns}))
			} else {
				Expect(inspect.InspectPodToJSON().SharedNamespaces).To(BeNil())
			}
		})

	}
})
