package integration

import (
	"fmt"
	"os"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
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
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
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
})
