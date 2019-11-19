// +build !remoteclient

package integration

import (
	"os"
	"strings"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman build", func() {
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
	})

	// Let's first do the most simple build possible to make sure stuff is
	// happy and then clean up after ourselves to make sure that works too.
	It("podman build and remove basic alpine", func() {
		session := podmanTest.PodmanNoCache([]string{"build", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	// If the context directory is pointing at a file and not a directory,
	// that's a no no, fail out.
	It("podman build context directory a file", func() {
		session := podmanTest.PodmanNoCache([]string{"build", "build/context_dir_a_file"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	// Check that builds with different values for the squash options
	// create the appropriate number of layers, then clean up after.
	It("podman build basic alpine with squash", func() {
		session := podmanTest.PodmanNoCache([]string{"build", "-f", "build/squash/Dockerfile.squash-a", "-t", "test-squash-a:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// Check for two layers
		Expect(len(strings.Fields(session.OutputToString()))).To(Equal(2))

		session = podmanTest.PodmanNoCache([]string{"build", "-f", "build/squash/Dockerfile.squash-b", "--squash", "-t", "test-squash-b:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-b"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// Check for three layers
		Expect(len(strings.Fields(session.OutputToString()))).To(Equal(3))

		session = podmanTest.PodmanNoCache([]string{"build", "-f", "build/squash/Dockerfile.squash-c", "--squash", "-t", "test-squash-c:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-c"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// Check for two layers
		Expect(len(strings.Fields(session.OutputToString()))).To(Equal(2))

		session = podmanTest.PodmanNoCache([]string{"build", "-f", "build/squash/Dockerfile.squash-c", "--squash-all", "-t", "test-squash-d:latest", "build/squash"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RootFS.Layers}}", "test-squash-d"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// Check for one layers
		Expect(len(strings.Fields(session.OutputToString()))).To(Equal(1))

		session = podmanTest.PodmanNoCache([]string{"rm", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"rmi", "-a", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
})
