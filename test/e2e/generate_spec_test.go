package integration

import (
	"os"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman generate spec", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRemote("podman generate spec is not supported on the remote client")
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

	It("podman generate spec bogus should fail", func() {
		session := podmanTest.Podman([]string{"generate", "spec", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError())
	})

	It("podman generate spec basic usage", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		session := podmanTest.Podman([]string{"create", "--cpus", "5", "--name", "specgen", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"generate", "spec", "--compact", "specgen"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman generate spec file", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		session := podmanTest.Podman([]string{"create", "--cpus", "5", "--name", "specgen", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"generate", "spec", "--filename", filepath.Join(tempdir, "out.json"), "specgen"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		path := filepath.Join(tempdir, "out.json")

		exec := SystemExec("cat", []string{path})
		exec.WaitWithDefaultTimeout()
		Expect(exec.OutputToString()).Should(ContainSubstring("specgen-clone"))
		Expect(exec.OutputToString()).Should(ContainSubstring("500000"))

	})

	It("generate spec pod", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--cpus", "5", "--name", "podspecgen"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"generate", "spec", "--compact", "podspecgen"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
})
