package integration

import (
	"fmt"
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman rename", func() {
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

	It("podman rename on non-existent container", func() {
		session := podmanTest.Podman([]string{"rename", "doesNotExist", "aNewName"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("Podman rename on existing container with bad name", func() {
		ctrName := "testCtr"
		ctr := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		newName := "invalid<>:char"
		rename := podmanTest.Podman([]string{"rename", ctrName, newName})
		rename.WaitWithDefaultTimeout()
		Expect(rename).To(ExitWithError())

		ps := podmanTest.Podman([]string{"ps", "-aq", "--filter", fmt.Sprintf("name=%s", ctrName), "--format", "{{ .Names }}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(Exit(0))
		Expect(ps.OutputToString()).To(ContainSubstring(ctrName))
	})

	It("Successfully rename a created container", func() {
		ctrName := "testCtr"
		ctr := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		newName := "aNewName"
		rename := podmanTest.Podman([]string{"rename", ctrName, newName})
		rename.WaitWithDefaultTimeout()
		Expect(rename).Should(Exit(0))

		ps := podmanTest.Podman([]string{"ps", "-aq", "--filter", fmt.Sprintf("name=%s", newName), "--format", "{{ .Names }}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(Exit(0))
		Expect(ps.OutputToString()).To(ContainSubstring(newName))
	})

	It("Successfully rename a created container and test event generated", func() {
		ctrName := "testCtr"
		ctr := podmanTest.Podman([]string{"create", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		newName := "aNewName"
		rename := podmanTest.Podman([]string{"rename", ctrName, newName})
		rename.WaitWithDefaultTimeout()
		Expect(rename).Should(Exit(0))

		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "container=aNewName"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring("rename"))
	})

	It("Successfully rename a running container", func() {
		ctrName := "testCtr"
		ctr := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		newName := "aNewName"
		rename := podmanTest.Podman([]string{"rename", ctrName, newName})
		rename.WaitWithDefaultTimeout()
		Expect(rename).Should(Exit(0))

		ps := podmanTest.Podman([]string{"ps", "-aq", "--filter", fmt.Sprintf("name=%s", newName), "--format", "{{ .Names }}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(Exit(0))
		Expect(ps.OutputToString()).To(ContainSubstring(newName))
	})

	It("Rename a running container with exec sessions", func() {
		ctrName := "testCtr"
		ctr := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		exec := podmanTest.Podman([]string{"exec", "-d", ctrName, "top"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))

		newName := "aNewName"
		rename := podmanTest.Podman([]string{"rename", ctrName, newName})
		rename.WaitWithDefaultTimeout()
		Expect(rename).Should(Exit(0))

		ps := podmanTest.Podman([]string{"ps", "-aq", "--filter", fmt.Sprintf("name=%s", newName), "--format", "{{ .Names }}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(Exit(0))
		Expect(ps.OutputToString()).To(ContainSubstring(newName))
	})

	It("Rename a container that is part of a pod", func() {
		podName := "testPod"
		infraName := "infra1"
		pod := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--infra-name", infraName})
		pod.WaitWithDefaultTimeout()
		Expect(pod).Should(Exit(0))

		infraName2 := "infra2"
		rename := podmanTest.Podman([]string{"rename", infraName, infraName2})
		rename.WaitWithDefaultTimeout()
		Expect(rename).Should(Exit(0))

		remove := podmanTest.Podman([]string{"pod", "rm", "-f", podName})
		remove.WaitWithDefaultTimeout()
		Expect(remove).Should(Exit(0))

		create := podmanTest.Podman([]string{"create", "--name", infraName2, ALPINE, "top"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		create2 := podmanTest.Podman([]string{"create", "--name", infraName, ALPINE, "top"})
		create2.WaitWithDefaultTimeout()
		Expect(create2).Should(Exit(0))
	})
})
