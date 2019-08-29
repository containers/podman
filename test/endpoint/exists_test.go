package endpoint

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman exists", func() {
	var (
		tempdir      string
		err          error
		endpointTest *EndpointTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		endpointTest = Setup(tempdir)
		endpointTest.StartVarlinkWithCache()
	})

	AfterEach(func() {
		endpointTest.Cleanup()
		//f := CurrentGinkgoTestDescription()
		//processTestResult(f)

	})

	It("image exists in local storage", func() {
		result := endpointTest.Varlink("ImageExists", makeNameMessage(ALPINE), false)
		Expect(result.ExitCode()).To(BeZero())

		output := result.OutputToMapToInt()
		Expect(output["exists"]).To(BeZero())
	})

	It("image exists in local storage by shortname", func() {
		result := endpointTest.Varlink("ImageExists", makeNameMessage("alpine"), false)
		Expect(result.ExitCode()).To(BeZero())

		output := result.OutputToMapToInt()
		Expect(output["exists"]).To(BeZero())
	})

	It("image does not exist in local storage", func() {
		result := endpointTest.Varlink("ImageExists", makeNameMessage("alpineforest"), false)
		Expect(result.ExitCode()).To(BeZero())

		output := result.OutputToMapToInt()
		Expect(output["exists"]).To(Equal(1))
	})

	It("container exists in local storage by name", func() {
		_ = endpointTest.startTopContainer("top")
		result := endpointTest.Varlink("ContainerExists", makeNameMessage("top"), false)
		Expect(result.ExitCode()).To(BeZero())
		output := result.OutputToMapToInt()
		Expect(output["exists"]).To(BeZero())
	})

})
