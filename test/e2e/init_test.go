package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman init bogus container", func() {
		session := podmanTest.Podman([]string{"start", "123456"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman init with no arguments", func() {
		session := podmanTest.Podman([]string{"start"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman init single container by ID", func() {
		session := podmanTest.Podman([]string{"create", "-d", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()
		init := podmanTest.Podman([]string{"init", cid})
		init.WaitWithDefaultTimeout()
		Expect(init.ExitCode()).To(Equal(0))
		result := podmanTest.Podman([]string{"inspect", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		conData := result.InspectContainerToJSON()
		Expect(conData[0].State.Status).To(Equal("created"))
	})

	It("podman init single container by name", func() {
		name := "test1"
		session := podmanTest.Podman([]string{"create", "--name", name, "-d", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		init := podmanTest.Podman([]string{"init", name})
		init.WaitWithDefaultTimeout()
		Expect(init.ExitCode()).To(Equal(0))
		result := podmanTest.Podman([]string{"inspect", name})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		conData := result.InspectContainerToJSON()
		Expect(conData[0].State.Status).To(Equal("created"))
	})

	It("podman init latest container", func() {
		SkipIfRemote()
		session := podmanTest.Podman([]string{"create", "-d", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		init := podmanTest.Podman([]string{"init", "--latest"})
		init.WaitWithDefaultTimeout()
		Expect(init.ExitCode()).To(Equal(0))
		result := podmanTest.Podman([]string{"inspect", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		conData := result.InspectContainerToJSON()
		Expect(conData[0].State.Status).To(Equal("created"))
	})

	It("podman init all three containers, one running", func() {
		Skip(v2remotefail)
		session := podmanTest.Podman([]string{"create", "--name", "test1", "-d", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session2 := podmanTest.Podman([]string{"create", "--name", "test2", "-d", ALPINE, "ls"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))
		session3 := podmanTest.Podman([]string{"run", "--name", "test3", "-d", ALPINE, "top"})
		session3.WaitWithDefaultTimeout()
		Expect(session3.ExitCode()).To(Equal(0))
		init := podmanTest.Podman([]string{"init", "--all"})
		init.WaitWithDefaultTimeout()
		Expect(init.ExitCode()).To(Equal(0))
		result := podmanTest.Podman([]string{"inspect", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		conData := result.InspectContainerToJSON()
		Expect(conData[0].State.Status).To(Equal("created"))
		result2 := podmanTest.Podman([]string{"inspect", "test2"})
		result2.WaitWithDefaultTimeout()
		Expect(result2.ExitCode()).To(Equal(0))
		conData2 := result2.InspectContainerToJSON()
		Expect(conData2[0].State.Status).To(Equal("created"))
		result3 := podmanTest.Podman([]string{"inspect", "test3"})
		result3.WaitWithDefaultTimeout()
		Expect(result3.ExitCode()).To(Equal(0))
		conData3 := result3.InspectContainerToJSON()
		Expect(conData3[0].State.Status).To(Equal("running"))
	})

	It("podman init running container errors", func() {
		Skip(v2remotefail)
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		init := podmanTest.Podman([]string{"init", "--latest"})
		init.WaitWithDefaultTimeout()
		Expect(init.ExitCode()).To(Equal(125))
	})
})
