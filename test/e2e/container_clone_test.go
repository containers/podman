//go:build linux || freebsd

package integration

import (
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman container clone", func() {
	BeforeEach(func() {
		SkipIfRemote("podman container clone is not supported in remote")
	})

	It("podman container clone basic test", func() {
		SkipIfRootlessCgroupsV1("starting a container with the memory limits not supported")
		create := podmanTest.Podman([]string{"create", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(ExitCleanly())
		clone := podmanTest.Podman([]string{"container", "clone", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(ExitCleanly())

		clone = podmanTest.Podman([]string{"container", "clone", clone.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(ExitCleanly())

		ctrInspect := podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).To(ExitCleanly())
		Expect(ctrInspect.InspectContainerToJSON()[0].Name).To(ContainSubstring("-clone1"))

		ctrStart := podmanTest.Podman([]string{"container", "start", clone.OutputToString()})
		ctrStart.WaitWithDefaultTimeout()
		Expect(ctrStart).To(ExitCleanly())
	})

	It("podman container clone image test", func() {
		create := podmanTest.Podman([]string{"create", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(ExitCleanly())
		clone := podmanTest.Podman([]string{"container", "clone", create.OutputToString(), "new_name", fedoraMinimal})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(ExitCleanly())

		ctrInspect := podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).To(ExitCleanly())
		Expect(ctrInspect.InspectContainerToJSON()[0]).To(HaveField("ImageName", fedoraMinimal))
		Expect(ctrInspect.InspectContainerToJSON()[0]).To(HaveField("Name", "new_name"))
	})

	It("podman container clone name test", func() {
		create := podmanTest.Podman([]string{"create", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(ExitCleanly())
		clone := podmanTest.Podman([]string{"container", "clone", "--name", "testing123", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(ExitCleanly())

		cloneInspect := podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		cloneInspect.WaitWithDefaultTimeout()
		Expect(cloneInspect).To(ExitCleanly())
		cloneData := cloneInspect.InspectContainerToJSON()
		Expect(cloneData[0]).To(HaveField("Name", "testing123"))
	})

	It("podman container clone resource limits override", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		create := podmanTest.Podman([]string{"create", "--cpus=5", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(ExitCleanly())
		clone := podmanTest.Podman([]string{"container", "clone", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(ExitCleanly())

		createInspect := podmanTest.Podman([]string{"inspect", create.OutputToString()})
		createInspect.WaitWithDefaultTimeout()
		Expect(createInspect).To(ExitCleanly())
		createData := createInspect.InspectContainerToJSON()

		cloneInspect := podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		cloneInspect.WaitWithDefaultTimeout()
		Expect(cloneInspect).To(ExitCleanly())
		cloneData := cloneInspect.InspectContainerToJSON()
		Expect(createData[0].HostConfig).To(HaveField("NanoCpus", cloneData[0].HostConfig.NanoCpus))

		create = podmanTest.Podman([]string{"create", "--memory=5", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(ExitCleanly())
		clone = podmanTest.Podman([]string{"container", "clone", "--cpus=6", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(ExitCleanly())

		createInspect = podmanTest.Podman([]string{"inspect", create.OutputToString()})
		createInspect.WaitWithDefaultTimeout()
		Expect(createInspect).To(ExitCleanly())
		createData = createInspect.InspectContainerToJSON()

		cloneInspect = podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		cloneInspect.WaitWithDefaultTimeout()
		Expect(cloneInspect).To(ExitCleanly())
		cloneData = cloneInspect.InspectContainerToJSON()
		Expect(createData[0].HostConfig).To(HaveField("MemorySwap", cloneData[0].HostConfig.MemorySwap))

		create = podmanTest.Podman([]string{"create", "--cpus=5", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(ExitCleanly())
		clone = podmanTest.Podman([]string{"container", "clone", "--cpus=4", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(ExitCleanly())

		var nanoCPUs int64
		numCpus := 4
		nanoCPUs = int64(numCpus * 1000000000)

		createInspect = podmanTest.Podman([]string{"inspect", create.OutputToString()})
		createInspect.WaitWithDefaultTimeout()
		Expect(createInspect).To(ExitCleanly())
		createData = createInspect.InspectContainerToJSON()

		cloneInspect = podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		cloneInspect.WaitWithDefaultTimeout()
		Expect(cloneInspect).To(ExitCleanly())
		cloneData = cloneInspect.InspectContainerToJSON()
		Expect(createData[0].HostConfig.NanoCpus).ToNot(Equal(cloneData[0].HostConfig.NanoCpus))
		Expect(cloneData[0].HostConfig).To(HaveField("NanoCpus", nanoCPUs))

		create = podmanTest.Podman([]string{"create", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(ExitCleanly())
		clone = podmanTest.Podman([]string{"container", "clone", "--cpus=4", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(ExitCleanly())

		cloneInspect = podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		cloneInspect.WaitWithDefaultTimeout()
		Expect(cloneInspect).To(ExitCleanly())
		cloneData = cloneInspect.InspectContainerToJSON()
		Expect(cloneData[0].HostConfig).To(HaveField("MemorySwappiness", int64(0)))

	})

	It("podman container clone in a pod", func() {
		SkipIfRootlessCgroupsV1("starting a container with the memory limits not supported")
		run := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:1234", ALPINE, "sleep", "20"})
		run.WaitWithDefaultTimeout()
		Expect(run).To(ExitCleanly())
		clone := podmanTest.Podman([]string{"container", "clone", run.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(ExitCleanly())
		ctrStart := podmanTest.Podman([]string{"container", "start", clone.OutputToString()})
		ctrStart.WaitWithDefaultTimeout()
		Expect(ctrStart).To(ExitCleanly())

		checkClone := podmanTest.Podman([]string{"ps", "-f", "id=" + clone.OutputToString(), "--ns", "--format", "{{.Namespaces.IPC}} {{.Namespaces.UTS}} {{.Namespaces.NET}}"})
		checkClone.WaitWithDefaultTimeout()
		Expect(checkClone).Should(ExitCleanly())
		cloneArray := checkClone.OutputToStringArray()

		checkCreate := podmanTest.Podman([]string{"ps", "-f", "id=" + run.OutputToString(), "--ns", "--format", "{{.Namespaces.IPC}} {{.Namespaces.UTS}} {{.Namespaces.NET}}"})
		checkCreate.WaitWithDefaultTimeout()
		Expect(checkCreate).Should(ExitCleanly())
		createArray := checkCreate.OutputToStringArray()

		Expect(cloneArray).To(ContainElements(createArray))

		ctrInspect := podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(ExitCleanly())

		runInspect := podmanTest.Podman([]string{"inspect", run.OutputToString()})
		runInspect.WaitWithDefaultTimeout()
		Expect(runInspect).Should(ExitCleanly())

		Expect(ctrInspect.InspectContainerToJSON()[0].Pod).Should(Equal(runInspect.InspectContainerToJSON()[0].Pod))
		Expect(ctrInspect.InspectContainerToJSON()[0].HostConfig.NetworkMode).Should(Equal(runInspect.InspectContainerToJSON()[0].HostConfig.NetworkMode))
	})

	It("podman container clone to a pod", func() {
		createPod := podmanTest.Podman([]string{"pod", "create", "--share", "uts", "--name", "foo-pod"})
		createPod.WaitWithDefaultTimeout()
		Expect(createPod).To(ExitCleanly())

		ctr := podmanTest.RunTopContainer("ctr")
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		clone := podmanTest.Podman([]string{"container", "clone", "--name", "cloned", "--pod", "foo-pod", "ctr"})
		clone.WaitWithDefaultTimeout()
		// Cannot use ExitCleanly() because of warning: "At least one namespace was reset to the default configuration"
		Expect(clone).To(Exit(0))

		ctrInspect := podmanTest.Podman([]string{"inspect", "cloned"})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(ExitCleanly())

		Expect(ctrInspect.InspectContainerToJSON()[0].Pod).Should(Equal(createPod.OutputToString()))

		Expect(ctrInspect.InspectContainerToJSON()[0].HostConfig.NetworkMode).Should(Not(ContainSubstring("container:")))

		createPod = podmanTest.Podman([]string{"pod", "create", "--share", "uts,net", "--name", "bar-pod"})
		createPod.WaitWithDefaultTimeout()
		Expect(createPod).To(ExitCleanly())

		clone = podmanTest.Podman([]string{"container", "clone", "--name", "cloned2", "--pod", "bar-pod", "ctr"})
		clone.WaitWithDefaultTimeout()
		// Cannot use ExitCleanly() because of warning: "At least one namespace was reset to the default configuration"
		Expect(clone).To(Exit(0))

		ctrInspect = podmanTest.Podman([]string{"inspect", "cloned2"})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(ExitCleanly())

		Expect(ctrInspect.InspectContainerToJSON()[0].Pod).Should(Equal(createPod.OutputToString()))

		Expect(ctrInspect.InspectContainerToJSON()[0].HostConfig.NetworkMode).Should(ContainSubstring("container:"))
	})

	It("podman container clone --destroy --force test", func() {
		create := podmanTest.Podman([]string{"create", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).To(ExitCleanly())
		clone := podmanTest.Podman([]string{"container", "clone", "--destroy", create.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", create.OutputToString()})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).ToNot(ExitCleanly())

		run := podmanTest.Podman([]string{"run", "-dt", ALPINE, "top"})
		run.WaitWithDefaultTimeout()
		Expect(run).To(ExitCleanly())
		clone = podmanTest.Podman([]string{"container", "clone", "--destroy", "-f", run.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(ExitCleanly())

		inspect = podmanTest.Podman([]string{"inspect", run.OutputToString()})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).ToNot(ExitCleanly())

		run = podmanTest.Podman([]string{"run", "-dt", ALPINE})
		run.WaitWithDefaultTimeout()
		Expect(run).To(ExitCleanly())
		clone = podmanTest.Podman([]string{"container", "clone", "-f", run.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).ToNot(ExitCleanly())

	})

	It("podman container clone network passing", func() {
		networkCreate := podmanTest.Podman([]string{"network", "create", "testing123"})
		networkCreate.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork("testing123")
		Expect(networkCreate).To(ExitCleanly())
		run := podmanTest.Podman([]string{"run", "--network", "bridge", "-dt", ALPINE})
		run.WaitWithDefaultTimeout()
		Expect(run).To(ExitCleanly())

		connect := podmanTest.Podman([]string{"network", "connect", "testing123", run.OutputToString()})
		connect.WaitWithDefaultTimeout()
		Expect(connect).To(ExitCleanly())

		clone := podmanTest.Podman([]string{"container", "clone", run.OutputToString()})
		clone.WaitWithDefaultTimeout()
		Expect(clone).To(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", clone.OutputToString()})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).To(ExitCleanly())
		networks := inspect.InspectContainerToJSON()[0].NetworkSettings.Networks
		Expect(networks).To(HaveLen(2))
		Expect(networks).To(HaveKey("testing123"))
	})

	It("podman container clone env test", func() {
		session := podmanTest.Podman([]string{"run", "--name", "env_ctr", "-e", "ENV_TEST=123", ALPINE, "printenv", "ENV_TEST"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"container", "clone", "env_ctr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"start", "-a", "env_ctr-clone"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("123"))

		session = podmanTest.Podman([]string{"run", "--name", "env_ctr2", "-e", "ENV_TEST=12=3", ALPINE, "printenv", "ENV_TEST"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"container", "clone", "env_ctr2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"start", "-a", "env_ctr2-clone"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("12=3"))

	})
})
