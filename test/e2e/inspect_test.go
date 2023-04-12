package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/opencontainers/selinux/go-selinux"
)

var _ = Describe("Podman inspect", func() {
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

	It("podman inspect alpine image", func() {
		session := podmanTest.Podman([]string{"inspect", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(BeValidJSON())
		imageData := session.InspectImageJSON()
		Expect(imageData[0].RepoTags[0]).To(Equal("quay.io/libpod/alpine:latest"))
	})

	It("podman inspect bogus container", func() {
		session := podmanTest.Podman([]string{"inspect", "foobar4321"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman inspect filter should work if result contains tab", func() {
		session := podmanTest.Podman([]string{"build", "--tag", "envwithtab", "build/envwithtab"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Verify that OS and Arch are being set
		inspect := podmanTest.Podman([]string{"inspect", "-f", "{{ .Config.Env }}", "envwithtab"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		// output should not be empty
		// test validates fix for https://github.com/containers/podman/issues/8785
		Expect(inspect.OutputToString()).To(ContainSubstring("TEST="), ".Config.Env")

		session = podmanTest.Podman([]string{"rmi", "envwithtab"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman inspect with GO format", func() {
		session := podmanTest.Podman([]string{"inspect", "--format", "{{.ID}}", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"images", "-q", "--no-trunc", ALPINE})
		result.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(result.OutputToStringArray()).To(ContainElement("sha256:"+session.OutputToString()), "'podman images -q --no-truncate' includes 'podman inspect --format .ID'")
	})

	It("podman inspect specified type", func() {
		session := podmanTest.Podman([]string{"inspect", "--type", "image", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman inspect container with GO format for ConmonPidFile", func() {
		session, ec, _ := podmanTest.RunLsContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(ec).To(Equal(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.ConmonPidFile}}", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman inspect container with size", func() {
		session, ec, _ := podmanTest.RunLsContainer("sizetest")
		session.WaitWithDefaultTimeout()
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"inspect", "--size", "sizetest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		conData := result.InspectContainerToJSON()
		Expect(conData[0].SizeRootFs).To(BeNumerically(">", 0))
		Expect(*conData[0].SizeRw).To(BeNumerically(">=", 0))
	})

	It("podman inspect container and image", func() {
		ls, ec, _ := podmanTest.RunLsContainer("")
		ls.WaitWithDefaultTimeout()
		Expect(ec).To(Equal(0))
		cid := ls.OutputToString()

		result := podmanTest.Podman([]string{"inspect", "--format={{.ID}}", cid, ALPINE})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman inspect container and filter for Image{ID}", func() {
		ls, ec, _ := podmanTest.RunLsContainer("")
		ls.WaitWithDefaultTimeout()
		Expect(ec).To(Equal(0))
		cid := ls.OutputToString()

		result := podmanTest.Podman([]string{"inspect", "--format={{.ImageID}}", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).To(HaveLen(1))

		result = podmanTest.Podman([]string{"inspect", "--format={{.Image}}", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman inspect container and filter for CreateCommand", func() {
		ls, ec, _ := podmanTest.RunLsContainer("")
		ls.WaitWithDefaultTimeout()
		Expect(ec).To(Equal(0))
		cid := ls.OutputToString()

		result := podmanTest.Podman([]string{"inspect", "--format={{.Config.CreateCommand}}", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman inspect -l with additional input should fail", func() {
		SkipIfRemote("--latest flag n/a")
		result := podmanTest.Podman([]string{"inspect", "-l", "1234foobar"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
	})

	It("podman inspect with mount filters", func() {

		ctrSession := podmanTest.Podman([]string{"create", "--name", "test", "-v", "/tmp:/test1", ALPINE, "top"})
		ctrSession.WaitWithDefaultTimeout()
		Expect(ctrSession).Should(Exit(0))

		inspectSource := podmanTest.Podman([]string{"inspect", "test", "--format", "{{(index .Mounts 0).Source}}"})
		inspectSource.WaitWithDefaultTimeout()
		Expect(inspectSource).Should(Exit(0))
		Expect(inspectSource.OutputToString()).To(Equal("/tmp"))

		inspectSrc := podmanTest.Podman([]string{"inspect", "test", "--format", "{{(index .Mounts 0).Src}}"})
		inspectSrc.WaitWithDefaultTimeout()
		Expect(inspectSrc).Should(Exit(0))
		Expect(inspectSrc.OutputToString()).To(Equal("/tmp"))

		inspectDestination := podmanTest.Podman([]string{"inspect", "test", "--format", "{{(index .Mounts 0).Destination}}"})
		inspectDestination.WaitWithDefaultTimeout()
		Expect(inspectDestination).Should(Exit(0))
		Expect(inspectDestination.OutputToString()).To(Equal("/test1"))

		inspectDst := podmanTest.Podman([]string{"inspect", "test", "--format", "{{(index .Mounts 0).Dst}}"})
		inspectDst.WaitWithDefaultTimeout()
		Expect(inspectDst).Should(Exit(0))
		Expect(inspectDst.OutputToString()).To(Equal("/test1"))
	})

	It("podman inspect shows healthcheck on docker image", func() {
		podmanTest.AddImageToRWStore(HEALTHCHECK_IMAGE)
		session := podmanTest.Podman([]string{"inspect", "--format=json", HEALTHCHECK_IMAGE})
		session.WaitWithDefaultTimeout()
		imageData := session.InspectImageJSON()
		Expect(imageData[0].HealthCheck.Timeout).To(BeNumerically("==", 3000000000))
		Expect(imageData[0].HealthCheck.Interval).To(BeNumerically("==", 60000000000))
		Expect(imageData[0].HealthCheck).To(HaveField("Test", []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"}))
	})

	It("podman inspect --latest with no container fails", func() {
		SkipIfRemote("testing --latest flag")

		session := podmanTest.Podman([]string{"inspect", "--latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman [image,container] inspect on image", func() {
		baseInspect := podmanTest.Podman([]string{"inspect", ALPINE})
		baseInspect.WaitWithDefaultTimeout()
		Expect(baseInspect).Should(Exit(0))
		baseJSON := baseInspect.InspectImageJSON()
		Expect(baseJSON).To(HaveLen(1))

		ctrInspect := podmanTest.Podman([]string{"container", "inspect", ALPINE})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).To(ExitWithError())

		imageInspect := podmanTest.Podman([]string{"image", "inspect", ALPINE})
		imageInspect.WaitWithDefaultTimeout()
		Expect(imageInspect).Should(Exit(0))
		imageJSON := imageInspect.InspectImageJSON()
		Expect(imageJSON).To(HaveLen(1))

		Expect(baseJSON[0]).To(HaveField("ID", imageJSON[0].ID))
	})

	It("podman [image, container] inspect on container", func() {
		ctrName := "testCtr"
		create := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE, "sh"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		baseInspect := podmanTest.Podman([]string{"inspect", ctrName})
		baseInspect.WaitWithDefaultTimeout()
		Expect(baseInspect).Should(Exit(0))
		baseJSON := baseInspect.InspectContainerToJSON()
		Expect(baseJSON).To(HaveLen(1))

		ctrInspect := podmanTest.Podman([]string{"container", "inspect", ctrName})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(Exit(0))
		ctrJSON := ctrInspect.InspectContainerToJSON()
		Expect(ctrJSON).To(HaveLen(1))

		imageInspect := podmanTest.Podman([]string{"image", "inspect", ctrName})
		imageInspect.WaitWithDefaultTimeout()
		Expect(imageInspect).To(ExitWithError())

		Expect(baseJSON[0]).To(HaveField("ID", ctrJSON[0].ID))
	})

	It("podman inspect always produces a valid array", func() {
		baseInspect := podmanTest.Podman([]string{"inspect", "doesNotExist"})
		baseInspect.WaitWithDefaultTimeout()
		Expect(baseInspect).To(ExitWithError())
		emptyJSON := baseInspect.InspectContainerToJSON()
		Expect(emptyJSON).To(BeEmpty())
	})

	It("podman inspect one container with not exist returns 1-length valid array", func() {
		ctrName := "testCtr"
		create := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE, "sh"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		baseInspect := podmanTest.Podman([]string{"inspect", ctrName, "doesNotExist"})
		baseInspect.WaitWithDefaultTimeout()
		Expect(baseInspect).To(ExitWithError())
		baseJSON := baseInspect.InspectContainerToJSON()
		Expect(baseJSON).To(HaveLen(1))
		Expect(baseJSON[0]).To(HaveField("Name", ctrName))
	})

	It("podman inspect container + image with same name gives container", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		ctrName := "testcontainer"
		create := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE, "sh"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		tag := podmanTest.Podman([]string{"tag", ALPINE, ctrName + ":latest"})
		tag.WaitWithDefaultTimeout()
		Expect(tag).Should(Exit(0))

		baseInspect := podmanTest.Podman([]string{"inspect", ctrName})
		baseInspect.WaitWithDefaultTimeout()
		Expect(baseInspect).Should(Exit(0))
		baseJSON := baseInspect.InspectContainerToJSON()
		Expect(baseJSON).To(HaveLen(1))
		Expect(baseJSON[0]).To(HaveField("Name", ctrName))
	})

	It("podman inspect - HostConfig.SecurityOpt ", func() {
		if !selinux.GetEnabled() {
			Skip("SELinux not enabled")
		}

		ctrName := "hugo"
		create := podmanTest.Podman([]string{
			"create", "--name", ctrName,
			"--security-opt", "seccomp=unconfined",
			"--security-opt", "label=type:spc_t",
			"--security-opt", "label=level:s0",
			ALPINE, "sh"})

		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		baseInspect := podmanTest.Podman([]string{"inspect", ctrName})
		baseInspect.WaitWithDefaultTimeout()
		Expect(baseInspect).Should(Exit(0))
		baseJSON := baseInspect.InspectContainerToJSON()
		Expect(baseJSON).To(HaveLen(1))
		Expect(baseJSON[0].HostConfig).To(HaveField("SecurityOpt", []string{"label=type:spc_t,label=level:s0", "seccomp=unconfined"}))
	})

	It("podman inspect pod", func() {
		podName := "testpod"
		create := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", podName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(BeValidJSON())
		podData := inspect.InspectPodArrToJSON()
		Expect(podData[0]).To(HaveField("Name", podName))
	})

	It("podman inspect pod with type", func() {
		podName := "testpod"
		create := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "--type", "pod", podName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(BeValidJSON())
		podData := inspect.InspectPodArrToJSON()
		Expect(podData[0]).To(HaveField("Name", podName))
	})

	It("podman inspect latest pod", func() {
		SkipIfRemote("--latest flag n/a")
		podName := "testpod"
		create := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "--type", "pod", "--latest"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(BeValidJSON())
		podData := inspect.InspectPodArrToJSON()
		Expect(podData[0]).To(HaveField("Name", podName))
	})
	It("podman inspect latest defaults to latest container", func() {
		SkipIfRemote("--latest flag n/a")
		podName := "testpod"
		pod := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		pod.WaitWithDefaultTimeout()
		Expect(pod).Should(Exit(0))

		inspect1 := podmanTest.Podman([]string{"inspect", "--type", "pod", podName})
		inspect1.WaitWithDefaultTimeout()
		Expect(inspect1).Should(Exit(0))
		Expect(inspect1.OutputToString()).To(BeValidJSON())
		podData := inspect1.InspectPodArrToJSON()
		infra := podData[0].Containers[0].Name

		inspect := podmanTest.Podman([]string{"inspect", "--latest"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(BeValidJSON())
		containerData := inspect.InspectContainerToJSON()
		Expect(containerData[0]).To(HaveField("Name", infra))
	})

	It("podman inspect network", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"inspect", name, "--format", "{{.Driver}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("bridge"))
	})

	It("podman inspect a volume", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"inspect", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(BeValidJSON())
	})

	It("podman inspect a volume with --format", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.Name}}", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(volName))
	})
	It("podman inspect --type container on a pod should fail", func() {
		podName := "testpod"
		create := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "--type", "container", podName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).To(ExitWithError())
	})

	It("podman inspect --type network on a container should fail", func() {
		ctrName := "testctr"
		create := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "--type", "network", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).To(ExitWithError())
	})

	It("podman inspect --type pod on a container should fail", func() {
		ctrName := "testctr"
		create := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "--type", "pod", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).To(ExitWithError())
	})

	It("podman inspect --type volume on a container should fail", func() {
		ctrName := "testctr"
		create := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "--type", "volume", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).To(ExitWithError())
	})

	// Fixes https://github.com/containers/podman/issues/8444
	It("podman inspect --format json .NetworkSettings.Ports", func() {
		ctnrName := "Ctnr_" + RandomString(25)

		create := podmanTest.Podman([]string{"create", "--name", ctnrName, "-p", "8084:80", ALPINE})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", `--format="{{json .NetworkSettings.Ports}}"`, ctnrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal(`"{"80/tcp":[{"HostIp":"","HostPort":"8084"}]}"`))
	})

	It("Verify container inspect has default network", func() {
		SkipIfRootless("Requires root CNI networking")
		ctrName := "testctr"
		session := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.InspectContainer(ctrName)
		Expect(inspect).To(HaveLen(1))
		Expect(inspect[0].NetworkSettings.Networks).To(HaveLen(1))
	})

	It("Verify stopped container still has default network in inspect", func() {
		SkipIfRootless("Requires root CNI networking")
		ctrName := "testctr"
		session := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.InspectContainer(ctrName)
		Expect(inspect).To(HaveLen(1))
		Expect(inspect[0].NetworkSettings.Networks).To(HaveLen(1))
	})

	It("Container inspect with unlimited uilimits should be -1", func() {
		ctrName := "testctr"
		session := podmanTest.Podman([]string{"run", "-d", "--ulimit", "core=-1:-1", "--name", ctrName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		data := inspect.InspectContainerToJSON()
		ulimits := data[0].HostConfig.Ulimits
		Expect(ulimits).ToNot(BeEmpty())
		found := false
		for _, ulimit := range ulimits {
			if ulimit.Name == "RLIMIT_CORE" {
				found = true
				Expect(ulimit.Soft).To(BeNumerically("==", -1))
				Expect(ulimit.Hard).To(BeNumerically("==", -1))
			}
		}
		Expect(found).To(BeTrue())
	})

	It("Dropped capabilities are sorted", func() {
		ctrName := "testCtr"
		session := podmanTest.Podman([]string{"run", "-d", "--cap-drop", "SETUID", "--cap-drop", "SETGID", "--cap-drop", "CAP_NET_BIND_SERVICE", "--name", ctrName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		data := inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].HostConfig.CapDrop).To(HaveLen(3))
		Expect(data[0].HostConfig.CapDrop[0]).To(Equal("CAP_NET_BIND_SERVICE"))
		Expect(data[0].HostConfig.CapDrop[1]).To(Equal("CAP_SETGID"))
		Expect(data[0].HostConfig.CapDrop[2]).To(Equal("CAP_SETUID"))
	})

	It("Add capabilities are sorted", func() {
		ctrName := "testCtr"
		session := podmanTest.Podman([]string{"run", "-d", "--cap-add", "SYS_ADMIN", "--cap-add", "CAP_NET_ADMIN", "--name", ctrName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		data := inspect.InspectContainerToJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].HostConfig.CapAdd).To(HaveLen(2))
		Expect(data[0].HostConfig.CapAdd[0]).To(Equal("CAP_NET_ADMIN"))
		Expect(data[0].HostConfig.CapAdd[1]).To(Equal("CAP_SYS_ADMIN"))
	})

	It("podman inspect container with GO format for PidFile", func() {
		SkipIfRemote("pidfile not handled by remote")
		session, ec, _ := podmanTest.RunLsContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(ec).To(Equal(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.PidFile}}", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman inspect container with bad create args", func() {
		session := podmanTest.Podman([]string{"container", "create", ALPINE, "efcho", "Hello World"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"container", "inspect", cid, "-f", "{{ .State.Error }}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(HaveLen(0))

		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		session = podmanTest.Podman([]string{"container", "inspect", cid, "-f", "'{{ .State.Error }}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(HaveLen(0)))
	})

})
