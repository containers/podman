package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman container clone", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRemote("podman container clone is not supported in remote")
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

	It("podman container clone basic test", func() {
		SkipIfRootlessCgroupsV1("starting a container with the memory limits not supported")
		create := podmanTest.Podman([]string{"create", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(Exit(0))
		clone := podmanTest.Podman([]string{"container", "clone", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		clone = podmanTest.Podman([]string{"container", "clone", clone.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		ctrInspect := podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).To(Exit(0))
		Expect(ctrInspect.InspectContainerToJSON()[0].Name).To(ContainSubstring("-clone1"))

		ctrStart := podmanTest.Podman([]string{"container", "start", clone.OutputToString()})
		ctrStart.WaitWithDefaultTimeout()
		Expect(ctrStart).To(Exit(0))
	})

	It("podman container clone image test", func() {
		create := podmanTest.Podman([]string{"create", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(Exit(0))
		clone := podmanTest.Podman([]string{"container", "clone", create.OutputToString(), "new_name", fedoraMinimal})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		ctrInspect := podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).To(Exit(0))
		Expect(ctrInspect.InspectContainerToJSON()[0].ImageName).To(Equal(fedoraMinimal))
		Expect(ctrInspect.InspectContainerToJSON()[0].Name).To(Equal("new_name"))
	})

	It("podman container clone name test", func() {
		create := podmanTest.Podman([]string{"create", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(Exit(0))
		clone := podmanTest.Podman([]string{"container", "clone", "--name", "testing123", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		cloneInspect := podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		cloneInspect.WaitWithDefaultTimeout()
		Expect(cloneInspect).To(Exit(0))
		cloneData := cloneInspect.InspectContainerToJSON()
		Expect(cloneData[0].Name).To(Equal("testing123"))
	})

	It("podman container clone resource limits override", func() {
		create := podmanTest.Podman([]string{"create", "--cpus=5", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(Exit(0))
		clone := podmanTest.Podman([]string{"container", "clone", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		createInspect := podmanTest.Podman([]string{"inspect", create.OutputToString()})
		createInspect.WaitWithDefaultTimeout()
		Expect(createInspect).To(Exit(0))
		createData := createInspect.InspectContainerToJSON()

		cloneInspect := podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		cloneInspect.WaitWithDefaultTimeout()
		Expect(cloneInspect).To(Exit(0))
		cloneData := cloneInspect.InspectContainerToJSON()
		Expect(createData[0].HostConfig.NanoCpus).To(Equal(cloneData[0].HostConfig.NanoCpus))

		create = podmanTest.Podman([]string{"create", "--memory=5", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(Exit(0))
		clone = podmanTest.Podman([]string{"container", "clone", "--cpus=6", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		createInspect = podmanTest.Podman([]string{"inspect", create.OutputToString()})
		createInspect.WaitWithDefaultTimeout()
		Expect(createInspect).To(Exit(0))
		createData = createInspect.InspectContainerToJSON()

		cloneInspect = podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		cloneInspect.WaitWithDefaultTimeout()
		Expect(cloneInspect).To(Exit(0))
		cloneData = cloneInspect.InspectContainerToJSON()
		Expect(createData[0].HostConfig.MemorySwap).To(Equal(cloneData[0].HostConfig.MemorySwap))

		create = podmanTest.Podman([]string{"create", "--cpus=5", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(Exit(0))
		clone = podmanTest.Podman([]string{"container", "clone", "--cpus=4", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		var nanoCPUs int64
		numCpus := 4
		nanoCPUs = int64(numCpus * 1000000000)

		createInspect = podmanTest.Podman([]string{"inspect", create.OutputToString()})
		createInspect.WaitWithDefaultTimeout()
		Expect(createInspect).To(Exit(0))
		createData = createInspect.InspectContainerToJSON()

		cloneInspect = podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		cloneInspect.WaitWithDefaultTimeout()
		Expect(cloneInspect).To(Exit(0))
		cloneData = cloneInspect.InspectContainerToJSON()
		Expect(createData[0].HostConfig.NanoCpus).ToNot(Equal(cloneData[0].HostConfig.NanoCpus))
		Expect(cloneData[0].HostConfig.NanoCpus).To(Equal(nanoCPUs))
	})

	It("podman container clone in a pod", func() {
		SkipIfRootlessCgroupsV1("starting a container with the memory limits not supported")
		run := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:1234", ALPINE, "sleep", "20"})
		run.WaitWithDefaultTimeout()
		Expect(run).To(Exit(0))
		clone := podmanTest.Podman([]string{"container", "clone", run.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))
		ctrStart := podmanTest.Podman([]string{"container", "start", clone.OutputToString()})
		ctrStart.WaitWithDefaultTimeout()
		Expect(ctrStart).To(Exit(0))

		checkClone := podmanTest.Podman([]string{"ps", "-f", "id=" + clone.OutputToString(), "--ns", "--format", "{{.Namespaces.IPC}} {{.Namespaces.UTS}} {{.Namespaces.NET}}"})
		checkClone.WaitWithDefaultTimeout()
		Expect(checkClone).Should(Exit(0))
		cloneArray := checkClone.OutputToStringArray()

		checkCreate := podmanTest.Podman([]string{"ps", "-f", "id=" + run.OutputToString(), "--ns", "--format", "{{.Namespaces.IPC}} {{.Namespaces.UTS}} {{.Namespaces.NET}}"})
		checkCreate.WaitWithDefaultTimeout()
		Expect(checkCreate).Should(Exit(0))
		createArray := checkCreate.OutputToStringArray()

		Expect(cloneArray).To(ContainElements(createArray[:]))

		ctrInspect := podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(Exit(0))

		runInspect := podmanTest.Podman([]string{"inspect", run.OutputToString()})
		runInspect.WaitWithDefaultTimeout()
		Expect(runInspect).Should(Exit(0))

		Expect(ctrInspect.InspectContainerToJSON()[0].Pod).Should(Equal(runInspect.InspectContainerToJSON()[0].Pod))
		Expect(ctrInspect.InspectContainerToJSON()[0].HostConfig.NetworkMode).Should(Equal(runInspect.InspectContainerToJSON()[0].HostConfig.NetworkMode))
	})

	It("podman container clone to a pod", func() {
		createPod := podmanTest.Podman([]string{"pod", "create", "--share", "uts", "--name", "foo-pod"})
		createPod.WaitWithDefaultTimeout()
		Expect(createPod).To(Exit(0))

		ctr := podmanTest.RunTopContainer("ctr")
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		clone := podmanTest.Podman([]string{"container", "clone", "--name", "cloned", "--pod", "foo-pod", "ctr"})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		ctrInspect := podmanTest.Podman([]string{"inspect", "cloned"})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(Exit(0))

		Expect(ctrInspect.InspectContainerToJSON()[0].Pod).Should(Equal(createPod.OutputToString()))

		Expect(ctrInspect.InspectContainerToJSON()[0].HostConfig.NetworkMode).Should(Not(ContainSubstring("container:")))

		createPod = podmanTest.Podman([]string{"pod", "create", "--share", "uts,net", "--name", "bar-pod"})
		createPod.WaitWithDefaultTimeout()
		Expect(createPod).To(Exit(0))

		clone = podmanTest.Podman([]string{"container", "clone", "--name", "cloned2", "--pod", "bar-pod", "ctr"})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(Exit(0))

		ctrInspect = podmanTest.Podman([]string{"inspect", "cloned2"})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(Exit(0))

		Expect(ctrInspect.InspectContainerToJSON()[0].Pod).Should(Equal(createPod.OutputToString()))

		Expect(ctrInspect.InspectContainerToJSON()[0].HostConfig.NetworkMode).Should(ContainSubstring("container:"))
	})
})
