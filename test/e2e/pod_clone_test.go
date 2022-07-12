package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman pod clone", func() {
	var (
		tempdir     string
		err         error
		podmanTest  *PodmanTestIntegration
		hostname, _ = os.Hostname()
	)

	BeforeEach(func() {
		SkipIfRemote("podman pod clone is not supported in remote")
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

	It("podman pod clone basic test", func() {
		create := podmanTest.Podman([]string{"pod", "create", "--name", "1"})
		create.WaitWithDefaultTimeout()
		Expect(create).To(Exit(0))

		run := podmanTest.Podman([]string{"run", "--pod", "1", "-dt", ALPINE})
		run.WaitWithDefaultTimeout()
		Expect(run).To(Exit(0))

		clone := podmanTest.Podman([]string{"pod", "clone", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		podInspect := podmanTest.Podman([]string{"pod", "inspect", clone.OutputToString()})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).To(Exit(0))
		data := podInspect.InspectPodToJSON()
		Expect(data.Name).To(ContainSubstring("-clone"))

		podStart := podmanTest.Podman([]string{"pod", "start", clone.OutputToString()})
		podStart.WaitWithDefaultTimeout()
		Expect(podStart).To(Exit(0))

		podInspect = podmanTest.Podman([]string{"pod", "inspect", clone.OutputToString()})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).To(Exit(0))
		data = podInspect.InspectPodToJSON()
		Expect(data.Containers[1].State).To(ContainSubstring("running")) // make sure non infra ctr is running
	})

	It("podman pod clone renaming test", func() {
		create := podmanTest.Podman([]string{"pod", "create", "--name", "1"})
		create.WaitWithDefaultTimeout()
		Expect(create).To(Exit(0))

		clone := podmanTest.Podman([]string{"pod", "clone", "--name", "2", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		podInspect := podmanTest.Podman([]string{"pod", "inspect", clone.OutputToString()})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).To(Exit(0))
		data := podInspect.InspectPodToJSON()
		Expect(data.Name).To(ContainSubstring("2"))

		podStart := podmanTest.Podman([]string{"pod", "start", clone.OutputToString()})
		podStart.WaitWithDefaultTimeout()
		Expect(podStart).To(Exit(0))
	})

	It("podman pod clone start test", func() {
		create := podmanTest.Podman([]string{"pod", "create", "--name", "1"})
		create.WaitWithDefaultTimeout()
		Expect(create).To(Exit(0))

		clone := podmanTest.Podman([]string{"pod", "clone", "--start", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		podInspect := podmanTest.Podman([]string{"pod", "inspect", clone.OutputToString()})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).To(Exit(0))
		data := podInspect.InspectPodToJSON()
		Expect(data.State).To(ContainSubstring("Running"))

	})

	It("podman pod clone destroy test", func() {
		create := podmanTest.Podman([]string{"pod", "create", "--name", "1"})
		create.WaitWithDefaultTimeout()
		Expect(create).To(Exit(0))

		clone := podmanTest.Podman([]string{"pod", "clone", "--destroy", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		podInspect := podmanTest.Podman([]string{"pod", "inspect", create.OutputToString()})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).ToNot(Exit(0))
	})

	It("podman pod clone infra option test", func() {
		// proof of concept that all currently tested infra options work since

		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(Exit(0))

		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podClone := podmanTest.Podman([]string{"pod", "clone", "--volume", volName + ":/tmp1", podName})
		podClone.WaitWithDefaultTimeout()
		Expect(podClone).Should(Exit(0))

		podInspect := podmanTest.Podman([]string{"pod", "inspect", "testPod-clone"})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(Exit(0))
		data := podInspect.InspectPodToJSON()
		Expect(data.Mounts[0]).To(HaveField("Name", volName))
	})

	It("podman pod clone --shm-size", func() {
		podCreate := podmanTest.Podman([]string{"pod", "create"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podClone := podmanTest.Podman([]string{"pod", "clone", "--shm-size", "10mb", podCreate.OutputToString()})
		podClone.WaitWithDefaultTimeout()
		Expect(podClone).Should(Exit(0))

		run := podmanTest.Podman([]string{"run", "-it", "--pod", podClone.OutputToString(), ALPINE, "mount"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		t, strings := run.GrepString("shm on /dev/shm type tmpfs")
		Expect(t).To(BeTrue())
		Expect(strings[0]).Should(ContainSubstring("size=10240k"))
	})

	It("podman pod clone --uts test", func() {
		SkipIfRemote("hostname for the custom NS test is not as expected on the remote client")

		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "clone", "--uts", "host", session.OutputToString()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "-it", "--pod", session.OutputToString(), ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(hostname))

		podName := "utsPod"
		ns := "ns:/proc/self/ns/"

		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// just share uts with a custom path
		podCreate := podmanTest.Podman([]string{"pod", "clone", "--uts", ns, "--name", podName, session.OutputToString()})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(Exit(0))

		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(Exit(0))
		podJSON := podInspect.InspectPodToJSON()
		Expect(podJSON.InfraConfig).To(HaveField("UtsNS", ns))
	})

})
