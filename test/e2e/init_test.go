package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman init", func() {
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
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman init bogus container", func() {
		session := podmanTest.Podman([]string{"start", "123456"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman init with no arguments", func() {
		session := podmanTest.Podman([]string{"start"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman init single container by ID", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		init := podmanTest.Podman([]string{"init", cid})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(Exit(0))
		Expect(init.OutputToString()).To(Equal(cid))
		result := podmanTest.Podman([]string{"inspect", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		conData := result.InspectContainerToJSON()
		Expect(conData[0].State).To(HaveField("Status", "initialized"))
	})

	It("podman init single container by name", func() {
		name := "test1"
		session := podmanTest.Podman([]string{"create", "--name", name, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		init := podmanTest.Podman([]string{"init", name})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(Exit(0))
		Expect(init.OutputToString()).To(Equal(name))
		result := podmanTest.Podman([]string{"inspect", name})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		conData := result.InspectContainerToJSON()
		Expect(conData[0].State).To(HaveField("Status", "initialized"))
	})

	It("podman init latest container", func() {
		SkipIfRemote("--latest flag n/a")
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		init := podmanTest.Podman([]string{"init", "--latest"})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(Exit(0))
		Expect(init.OutputToString()).To(Equal(cid))
		result := podmanTest.Podman([]string{"inspect", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		conData := result.InspectContainerToJSON()
		Expect(conData[0].State).To(HaveField("Status", "initialized"))
	})

	It("podman init all three containers, one running", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test1", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		session2 := podmanTest.Podman([]string{"create", "--name", "test2", ALPINE, "ls"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
		cid2 := session2.OutputToString()
		session3 := podmanTest.Podman([]string{"run", "--name", "test3", "-d", ALPINE, "top"})
		session3.WaitWithDefaultTimeout()
		Expect(session3).Should(Exit(0))
		cid3 := session3.OutputToString()
		init := podmanTest.Podman([]string{"init", "--all"})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(Exit(0))
		Expect(init.OutputToString()).To(ContainSubstring(cid))
		Expect(init.OutputToString()).To(ContainSubstring(cid2))
		Expect(init.OutputToString()).To(ContainSubstring(cid3))
		result := podmanTest.Podman([]string{"inspect", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		conData := result.InspectContainerToJSON()
		Expect(conData[0].State).To(HaveField("Status", "initialized"))
		result2 := podmanTest.Podman([]string{"inspect", "test2"})
		result2.WaitWithDefaultTimeout()
		Expect(result2).Should(Exit(0))
		conData2 := result2.InspectContainerToJSON()
		Expect(conData2[0].State).To(HaveField("Status", "initialized"))
		result3 := podmanTest.Podman([]string{"inspect", "test3"})
		result3.WaitWithDefaultTimeout()
		Expect(result3).Should(Exit(0))
		conData3 := result3.InspectContainerToJSON()
		Expect(conData3[0].State).To(HaveField("Status", "running"))
	})

	It("podman init running container errors", func() {
		session := podmanTest.Podman([]string{"run", "--name", "init_test", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		init := podmanTest.Podman([]string{"init", "init_test"})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(Exit(125))
	})
})
