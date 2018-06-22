package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run exit", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()

	})

	It("podman run -d mount cleanup test", func() {
		mount := podmanTest.SystemExec("mount", nil)
		mount.WaitWithDefaultTimeout()
		out1 := mount.OutputToString()
		result := podmanTest.Podman([]string{"run", "-d", ALPINE, "echo", "hello"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.SystemExec("sleep", []string{"5"})
		result.WaitWithDefaultTimeout()

		mount = podmanTest.SystemExec("mount", nil)
		mount.WaitWithDefaultTimeout()
		out2 := mount.OutputToString()
		Expect(out1).To(Equal(out2))
	})
})
