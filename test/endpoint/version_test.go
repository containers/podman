package endpoint

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	"github.com/containers/libpod/version"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman version", func() {
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

	It("podman version", func() {
		session := endpointTest.Varlink("GetVersion", "", false)
		result := session.OutputToStringMap()
		Expect(result["version"]).To(Equal(version.Version))
		Expect(session.ExitCode()).To(Equal(0))
	})
})
