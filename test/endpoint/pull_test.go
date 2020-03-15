package endpoint

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pull", func() {
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
		endpointTest.StartVarlink()
	})

	AfterEach(func() {
		endpointTest.Cleanup()
		//f := CurrentGinkgoTestDescription()
		//processTestResult(f)

	})

	It("podman pull", func() {
		session := endpointTest.Varlink("PullImage", makeNameMessage(ALPINE), false)
		Expect(session.ExitCode()).To(BeZero())

		result := endpointTest.Varlink("ImageExists", makeNameMessage(ALPINE), false)
		Expect(result.ExitCode()).To(BeZero())

		output := result.OutputToMapToInt()
		Expect(output["exists"]).To(BeZero())
	})
})
