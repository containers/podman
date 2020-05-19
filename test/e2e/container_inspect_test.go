package integration

import (
	"os"

	"github.com/containers/libpod/pkg/annotations"
	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman container inspect", func() {
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

	It("podman inspect a container for the container manager annotation", func() {
		const testContainer = "container-inspect-test-1"
		setup := podmanTest.RunTopContainer(testContainer)
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		data := podmanTest.InspectContainer(testContainer)
		Expect(data[0].Config.Annotations[annotations.ContainerManager]).
			To(Equal(annotations.ContainerManagerLibpod))
	})
})
